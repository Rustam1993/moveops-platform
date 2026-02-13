import { expect, test } from "@playwright/test";

function formatDate(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

test("MVP smoke: login -> estimate -> job -> calendar -> storage", async ({ page }) => {
  const suffix = Date.now().toString().slice(-6);
  const customerName = `E2E Customer ${suffix}`;
  const email = `e2e.${suffix}@example.com`;
  const moveDate = formatDate(new Date());

  await page.goto("/login");
  await page.getByLabel("Email").fill("admin@local.moveops");
  await page.getByLabel("Password").fill("Admin12345!");
  await Promise.all([
    page.waitForURL("**/"),
    page.getByRole("button", { name: "Sign in" }).click(),
  ]);

  await page.goto("/estimates/new");
  await page.getByLabel("Customer name").fill(customerName);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Primary phone").fill("5125550200");
  await page.getByLabel("Address").first().fill("100 Smoke Origin St");
  await page.getByLabel("City").first().fill("Austin");
  await page.getByLabel("State").first().fill("TX");
  await page.getByLabel("Postal code").first().fill("78701");
  await page.getByLabel("Address").nth(1).fill("900 Smoke Destination Ave");
  await page.getByLabel("City").nth(1).fill("Dallas");
  await page.getByLabel("State").nth(1).fill("TX");
  await page.getByLabel("Postal code").nth(1).fill("75001");
  await page.getByLabel("Move date").fill(moveDate);
  await page.getByLabel("Lead source").selectOption("Website");

  await Promise.all([
    page.waitForURL(/\/jobs\/.+/),
    page.getByRole("button", { name: "Convert to job" }).click(),
  ]);

  const headingText = await page.getByRole("heading", { name: /Job / }).innerText();
  const jobNumber = headingText.replace("Job ", "").trim();
  expect(jobNumber).toMatch(/^J-/);

  await page.goto("/calendar");
  await expect(page.getByText(jobNumber).first()).toBeVisible();

  await page.goto("/storage");
  await page.locator("header select").first().selectOption("Main Facility");
  await expect(page.getByText(jobNumber).first()).toBeVisible();
  await page.getByText(jobNumber).first().click();

  await expect(page.getByRole("heading", { name: /storage record/i })).toBeVisible();
  await page.locator("textarea").first().fill(`E2E note ${suffix}`);
  await page.getByRole("button", { name: "Save" }).click();

  await expect(page.getByText(jobNumber).first()).toBeVisible();
});
