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
  const loginResponsePromise = page.waitForResponse((response) => {
    return response.request().method() === "POST" && response.url().includes("/api/auth/login");
  });
  await page.getByRole("button", { name: "Sign in" }).click();
  const loginResponse = await loginResponsePromise;
  expect(loginResponse.ok()).toBeTruthy();
  await page.waitForURL(/\/$/);

  await page.goto("/estimates/new");
  await page.waitForURL(/\/estimates\/new$/);
  await expect(page.getByRole("heading", { name: "New Estimate" })).toBeVisible();
  await page.locator("#customerName").fill(customerName);
  await page.locator("#email").fill(email);
  await page.locator("#primaryPhone").fill("5125550200");
  await page.locator("#originAddressLine1").fill("100 Smoke Origin St");
  await page.locator("#originCity").fill("Austin");
  await page.locator("#originState").fill("TX");
  await page.locator("#originPostalCode").fill("78701");
  await page.locator("#destinationAddressLine1").fill("900 Smoke Destination Ave");
  await page.locator("#destinationCity").fill("Dallas");
  await page.locator("#destinationState").fill("TX");
  await page.locator("#destinationPostalCode").fill("75001");
  await page.locator("#moveDate").fill(moveDate);
  await page.locator("#leadSource").selectOption("Website");

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
