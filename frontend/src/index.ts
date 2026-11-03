import "./styles.css";

const frontendExample = document.querySelector<HTMLHeadingElement>("#frontend-example");
if (frontendExample === null) {
  throw new Error("frontend example element is missing");
}
frontendExample.textContent = "this is example frontend";
