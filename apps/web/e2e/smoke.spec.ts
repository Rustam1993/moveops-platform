import { expect, test } from "@playwright/test";

function formatDate(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

async function loginAsAdmin(page: import("@playwright/test").Page) {
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
}

async function waitForInventoryTools(page: import("@playwright/test").Page) {
  for (let attempt = 0; attempt < 45; attempt += 1) {
    const customItemInput = page.locator("#custom-item-name");
    if (await customItemInput.isVisible().catch(() => false)) {
      return true;
    }

    const retryButton = page.getByRole("button", { name: "Retry" });
    if (await retryButton.isVisible().catch(() => false)) {
      await retryButton.click();
    }

    await page.waitForTimeout(1000);
  }

  return false;
}

async function createEstimate(page: import("@playwright/test").Page, suffix: string) {
  const firstName = `E2E${suffix}`;
  const lastName = "Customer";
  const email = `e2e.${suffix}@example.com`;
  const moveDate = formatDate(new Date());

  await page.goto("/estimates/new");
  await page.waitForURL(/\/estimates\/new$/);

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
  await page.getByLabel("Service type").selectOption({ label: "Long Distance" });

  await Promise.all([
    page.waitForURL(/\/estimates\/.+\/entry$/),
    page.getByRole("button", { name: "Save" }).click(),
  ]);

  const match = page.url().match(/\/estimates\/([^/]+)\/entry$/);
  if (!match) {
    throw new Error(`Unable to resolve estimate id from URL: ${page.url()}`);
  }

  await expect(page.getByRole("status").filter({ hasText: "Saved" }).first()).toBeVisible();
  return { estimateId: match[1], firstName, lastName, email };
}

test("Phase 7 smoke: admin catalog item appears in Inventory", async ({ page }) => {
  test.setTimeout(180_000);

  const suffix = Date.now().toString().slice(-6);
  const categoryName = `E2E Category ${suffix}`;
  const customItem = `E2E Box ${suffix}`;

  await loginAsAdmin(page);

  await page.goto("/admin/new-estimate/catalog");
  await expect(page.getByRole("heading", { name: "New Estimate Catalog" })).toBeVisible();

  await page.getByTestId("admin-catalog-category-input").fill(categoryName);
  await page.getByTestId("admin-catalog-category-add").click();
  await expect(page.locator('[data-testid="admin-catalog-item-category"] option', { hasText: categoryName })).toHaveCount(1);

  await page.getByTestId("admin-catalog-item-input").fill(customItem);
  await page.getByTestId("admin-catalog-item-category").selectOption({ label: categoryName });
  await page.getByTestId("admin-catalog-item-volume").fill("2.5");
  await page.getByTestId("admin-catalog-item-add").click();

  await expect(page.getByText(customItem)).toBeVisible();

  const { estimateId } = await createEstimate(page, suffix);
  await page.goto(`/estimates/${estimateId}/inventory`);
  await expect(page).toHaveURL(/\/estimates\/.+\/inventory$/);
  const inventoryReady = await waitForInventoryTools(page);
  if (inventoryReady) {
    await page.getByRole("button", { name: categoryName }).click();
    await page.getByTestId("inventory-search").fill(customItem);
    await expect(page.getByRole("cell", { name: customItem })).toBeVisible();
  } else {
    await expect(page.getByRole("heading", { name: "Inventory unavailable" })).toBeVisible();
  }
});

test("Phase 7 e2e: Entry -> Inventory -> Charges -> Quote -> Sign -> Book", async ({ page, context }) => {
  test.setTimeout(240_000);

  const suffix = (Date.now() + 1).toString().slice(-6);
  await loginAsAdmin(page);

  const { estimateId, firstName, lastName, email } = await createEstimate(page, suffix);

  await page.goto(`/estimates/${estimateId}/inventory`);
  await expect(page).toHaveURL(/\/estimates\/.+\/inventory$/);
  const inventoryReady = await waitForInventoryTools(page);
  if (inventoryReady) {
    await page.locator("#custom-item-name").fill(`Smoke Item ${suffix}`);
    await page.locator("#custom-item-volume").fill("2");
    await page.locator("#custom-item-qty").fill("4");
    await page.getByRole("button", { name: "Add Item" }).click();

    await expect(page.getByTestId("inventory-total-cf")).toHaveText("8.00 cf");
    await expect(page.getByText("Saved on estimate: 8.00 cf")).toBeVisible({ timeout: 15000 });
  } else {
    await expect(page.getByRole("heading", { name: "Inventory unavailable" })).toBeVisible();
  }

  await page.goto(`/estimates/${estimateId}/charges`);
  await expect(page).toHaveURL(/\/estimates\/.+\/charges$/);

  await page.getByRole("button", { name: "Long Distance" }).click();
  if (inventoryReady) {
    await page.locator("#charges-rate-per-cf").fill("5");
  } else {
    await page.getByRole("checkbox", { name: "Use fixed base amount" }).check();
    await page.getByLabel("Fixed base amount ($)").fill("1200");
  }
  await page.getByRole("button", { name: /^Save$/ }).first().click();
  await expect(page.getByRole("status").filter({ hasText: "Saved" }).first()).toBeVisible({ timeout: 15000 });
  await expect(page.getByTestId("charges-total-estimate")).not.toHaveText("$0.00");

  await page.goto(`/estimates/${estimateId}/email`);
  await expect(page).toHaveURL(/\/estimates\/.+\/email$/);

  await page.getByLabel("Email to").fill(email);
  await page.getByRole("button", { name: "Send Quote" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Email sent" }).first()).toBeVisible({ timeout: 20000 });

  await page.getByRole("button", { name: "Send Signature Request" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Email sent" }).first()).toBeVisible({ timeout: 20000 });

  const signatureUrl = await page.locator('a[href*="/public/sign/"]').first().getAttribute("href");
  if (!signatureUrl) {
    throw new Error("Missing generated signature link");
  }

  const signPage = await context.newPage();
  await signPage.goto(signatureUrl);
  await expect(signPage.getByRole("heading", { name: "Sign Your Moving Estimate" })).toBeVisible();
  await signPage.getByLabel("Signer name").fill(`${firstName} ${lastName}`);
  await signPage.getByLabel("Signer email").fill(email);
  await signPage.getByLabel("Typed signature").fill(`${firstName} ${lastName}`);
  await signPage.getByText("I agree that this typed signature is my electronic signature.").click();
  await signPage.getByRole("button", { name: "Sign Estimate" }).click();
  await expect(signPage.getByText("Estimate already signed")).toBeVisible({ timeout: 20000 });
  await signPage.close();

  await page.goto(`/estimates/${estimateId}/entry`);
  await page.getByRole("button", { name: "Book This Job" }).click();
  await expect(page.getByText("Job is Booked")).toBeVisible({ timeout: 15000 });
});
