import { expect, test } from "@playwright/test";

function formatDate(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

test("Phase 3 smoke: login -> create estimate -> charges update persists", async ({ page }) => {
  const suffix = Date.now().toString().slice(-6);
  const firstName = `E2E${suffix}`;
  const lastName = "Customer";
  const updatedLastName = "Updated";
  const email = `e2e.${suffix}@example.com`;
  const moveDate = formatDate(new Date());

  await page.goto("/login");
  await page.getByLabel("Email").fill("admin@local.moveops");
  await page.getByLabel("Password").fill("Admin12345!");
  const loginResponsePromise = page.waitForResponse((response) => {
    return response.request().method() === "POST" && response.url().includes("/api/auth/login");
  });
  await page.getByRole("button", { name: "Sign in" }).click();
  const loginResponse = await loginResponsePromise;
  expect(loginResponse.ok()).toBeTruthy();
  await page.waitForURL(/\/$/);

  await page.goto("/estimates/new");
  await page.waitForURL(/\/estimates\/new$/);
  await expect(page.getByRole("heading", { name: "New estimate" })).toBeVisible();
  await expect(
    page
      .getByRole("tablist", { name: "Estimate workspace tabs" })
      .getByRole("button", { name: "Inventory", exact: true }),
  ).toBeDisabled();

  await page.getByLabel("First name").fill(firstName);
  await page.getByLabel("Last name").fill(lastName);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Phone").fill("5125550200");
  await page.getByLabel("Street").first().fill("100 Smoke Origin St");
  await page.getByLabel("City").first().fill("Austin");
  await page.getByLabel("State").first().fill("TX");
  await page.getByLabel("ZIP").first().fill("78701");
  await page.getByLabel("Street").nth(1).fill("900 Smoke Destination Ave");
  await page.getByLabel("City").nth(1).fill("Dallas");
  await page.getByLabel("State").nth(1).fill("TX");
  await page.getByLabel("ZIP").nth(1).fill("75001");
  await page.getByLabel("Move date").fill(moveDate);

  await Promise.all([
    page.waitForURL(/\/estimates\/.+\/entry$/),
    page.getByRole("button", { name: "Save" }).click(),
  ]);

  await expect(page.getByText("Saved")).toBeVisible();
  await expect(page.getByRole("link", { name: "Inventory" })).toBeVisible();

  await page.getByRole("link", { name: "Inventory" }).click();
  await expect(page).toHaveURL(/\/estimates\/.+\/inventory$/);

  await page.getByRole("link", { name: "Entry Form" }).click();
  await expect(page).toHaveURL(/\/estimates\/.+\/entry$/);
  await page.getByLabel("Last name").fill(updatedLastName);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Saved")).toBeVisible();

  await page.reload();
  await expect(page.getByLabel("Last name")).toHaveValue(updatedLastName);

  await page.getByRole("link", { name: "Charges" }).click();
  await expect(page).toHaveURL(/\/estimates\/.+\/charges$/);

  await page.getByLabel("# Workers").fill("2");
  await page.getByLabel("Labor hours").fill("3");
  await page.getByLabel("Labor rate ($/hr)").fill("150");
  await page.getByLabel("Travel hours").fill("1");
  await page.getByLabel("Travel rate ($/hr)").fill("150");

  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Saved")).toBeVisible();
  await expect(page.getByTestId("charges-total-estimate")).toHaveText("$1,050.00");

  await page.reload();
  await expect(page.getByLabel("Labor hours")).toHaveValue("3");
  await expect(page.getByTestId("charges-total-estimate")).toHaveText("$1,050.00");

  await page.getByRole("link", { name: "Printed Estimate" }).click();
  await expect(page).toHaveURL(/\/estimates\/.+\/printed-estimate$/);
  const generatePdfResponsePromise = page.waitForResponse((response) => {
    return (
      response.request().method() === "POST" &&
      response.url().includes("/documents/estimate-pdf")
    );
  });
  await page.getByRole("button", { name: "Generate PDF" }).click();
  const generatePdfResponse = await generatePdfResponsePromise;
  expect(generatePdfResponse.ok()).toBeTruthy();
  await expect(page.getByRole("button", { name: "Regenerate PDF" })).toBeVisible();
});
