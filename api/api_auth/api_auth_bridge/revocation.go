package api_auth_bridge

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RevocationStore 记录并查询「用户的登出时刻」——bridge 形态的本地令牌管理。
//
// 为什么桥接层必须本地管理令牌：JWT 无状态，IdP 的 logout 只吊销它那侧的
// refresh token；access token 在本服务的本地验签下仍有效到自然过期，
// 这一段只有本服务自己能拦。
//
// 按**用户维度**而不是逐 token 记：一次登出让该用户此前签发的全部令牌同时失效
// （多设备登出语义天然正确）；每用户一条记录不随签发次数增长；登出时无需持有
// access token。判定规则：令牌 iat < 该用户登出时刻 → 拒。
type RevocationStore interface {
	// MarkLoggedOut 记录 userID 在 at 时刻登出；retain 是记录保留时长
	// （应 >= IdP access token 最长寿命，到期自动回收）。
	MarkLoggedOut(ctx context.Context, userID string, at time.Time, retain time.Duration) error
	// LoggedOutAt 返回该用户最近一次登出时刻；无记录返回零值。
	LoggedOutAt(ctx context.Context, userID string) (time.Time, error)
}

var currentRevocation RevocationStore

// SetupRevocation 注入吊销存储（生产实例通常给 redis 等共享存储实现；
// 模板自带 MemoryRevocationStore 供单进程/测试）。未注入时桥接层只验签，
// 登出到令牌自然过期之间存在可用窗口（启动期日志应明示）。
func SetupRevocation(store RevocationStore) {
	currentRevocation = store
}

// RevocationConfigured 报告吊销存储是否已装配，供启动期日志与自检使用。
func RevocationConfigured() bool { return currentRevocation != nil }

// Revoked 判断令牌是否已被登出作废。查询失败按**拒绝**处理（fail-closed）：
// 存储不可用时宁可让用户重登，也不放行可能已登出的令牌。
// iat 缺席的令牌无法判定新旧，存在登出记录时按已吊销处理（IdP 必发 iat，缺席即异常令牌）。
func Revoked(ctx context.Context, userID string, issuedAt time.Time) (bool, error) {
	if currentRevocation == nil || userID == "" {
		return false, nil
	}
	loggedOutAt, err := currentRevocation.LoggedOutAt(ctx, userID)
	if err != nil {
		return true, fmt.Errorf("revocation lookup failed: %w", err)
	}
	if loggedOutAt.IsZero() {
		return false, nil
	}
	if issuedAt.IsZero() {
		return true, nil
	}
	return issuedAt.Before(loggedOutAt), nil
}

// RevokeUserTokens 记录一次登出。由业务的登出方法在转发 IdP 成功后调用。
func RevokeUserTokens(ctx context.Context, userID string, retain time.Duration) error {
	if currentRevocation == nil || userID == "" {
		return nil
	}
	return currentRevocation.MarkLoggedOut(ctx, userID, time.Now().UTC(), retain)
}

// MemoryRevocationStore 是进程内参考实现：单实例部署与测试可用；
// 多实例部署必须换共享存储实现（如 redis），否则各实例吊销状态不一致。
type MemoryRevocationStore struct {
	mu      sync.RWMutex
	entries map[string]memoryRevocationEntry
}

type memoryRevocationEntry struct {
	loggedOutAt time.Time
	expiresAt   time.Time
}

func NewMemoryRevocationStore() *MemoryRevocationStore {
	return &MemoryRevocationStore{entries: map[string]memoryRevocationEntry{}}
}

func (s *MemoryRevocationStore) MarkLoggedOut(_ context.Context, userID string, at time.Time, retain time.Duration) error {
	if retain <= 0 {
		retain = 24 * time.Hour
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[userID] = memoryRevocationEntry{
		loggedOutAt: at.UTC(),
		expiresAt:   time.Now().UTC().Add(retain),
	}
	return nil
}

func (s *MemoryRevocationStore) LoggedOutAt(_ context.Context, userID string) (time.Time, error) {
	s.mu.RLock()
	entry, ok := s.entries[userID]
	s.mu.RUnlock()
	if !ok {
		return time.Time{}, nil
	}
	if time.Now().UTC().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.entries, userID)
		s.mu.Unlock()
		return time.Time{}, nil
	}
	return entry.loggedOutAt, nil
}
