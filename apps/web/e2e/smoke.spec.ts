import { expect, test } from "@playwright/test";

function formatDate(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

async function waitForInventoryTools(page: import("@playwright/test").Page) {
  for (let attempt = 0; attempt < 12; attempt += 1) {
    const quickAdd = page.getByTestId("quick-add-starter-pack");
    if (await quickAdd.isVisible().catch(() => false)) {
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

test("Phase 3 smoke: login -> create estimate -> charges update persists", async ({ page }) => {
  test.setTimeout(180_000);

  const suffix = Date.now().toString().slice(-6);
  const firstName = `E2E${suffix}`;
  const lastName = "Customer";
  const updatedLastName = "Updated";
  const email = `e2e.${suffix}@example.com`;
  const moveDate = formatDate(new Date());

  await loginAsAdmin(page);

  await page.goto("/estimates/new");
  await page.waitForURL(/\/estimates\/new$/);
  await expect(page.getByRole("heading", { name: "New estimate" })).toBeVisible();
  await expect(page.getByTestId("readiness-state")).toHaveText("4 checks remaining");
  await expect(page.getByTestId("readiness-item-contact")).toContainText("Missing");
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
  const estimateIdMatch = page.url().match(/\/estimates\/([^/]+)\/entry$/);
  if (!estimateIdMatch) {
    throw new Error(`Unable to resolve estimate id from URL: ${page.url()}`);
  }
  const estimateId = estimateIdMatch[1];

  await expect(page.getByRole("status").filter({ hasText: "Saved" }).first()).toBeVisible();
  await expect(page.getByTestId("readiness-state")).toHaveText("2 checks remaining");
  await expect(page.getByRole("link", { name: "Inventory" })).toBeVisible();

  await page.goto(`/estimates/${estimateId}/inventory`);
  await expect(page).toHaveURL(/\/estimates\/.+\/inventory$/);
  const inventoryToolsReady = await waitForInventoryTools(page);
  if (inventoryToolsReady) {
    await page.getByTestId("quick-add-starter-pack").click();
    await expect(page.getByTestId("inventory-total-cf")).toHaveText("80.00 cf");
    await expect(page.getByText("Saved on estimate: 80.00 cf")).toBeVisible({ timeout: 15000 });
  } else {
    await expect(page.getByRole("heading", { name: "Inventory unavailable" })).toBeVisible();
  }

  await page.goto(`/estimates/${estimateId}/entry`);
  for (let attempt = 0; attempt < 2; attempt += 1) {
    if (await page.getByLabel("Last name").isVisible().catch(() => false)) {
      break;
    }
    await page.goto(`/estimates/${estimateId}/entry`);
    if (page.url().endsWith("/login")) {
      await loginAsAdmin(page);
      await page.goto(`/estimates/${estimateId}/entry`);
    }
  }
  await expect(page.getByLabel("Last name")).toBeVisible();
  await page.getByLabel("Last name").fill(updatedLastName);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Saved" }).first()).toBeVisible();

  await page.reload();
  await expect(page.getByLabel("Last name")).toHaveValue(updatedLastName);

  await page.goto(`/estimates/${estimateId}/charges`);
  for (let attempt = 0; attempt < 2; attempt += 1) {
    if (await page.getByLabel("# Workers").isVisible().catch(() => false)) {
      break;
    }
    await page.goto(`/estimates/${estimateId}/charges`);
    if (page.url().endsWith("/login")) {
      await loginAsAdmin(page);
      await page.goto(`/estimates/${estimateId}/charges`);
    }
  }
  await expect(page.getByLabel("# Workers")).toBeVisible();

  await page.getByLabel("# Workers").fill("2");
  await page.getByLabel("Labor hours").fill("3");
  await page.getByLabel("Labor rate ($/hr)").fill("150");
  await page.getByLabel("Travel hours").fill("1");
  await page.getByLabel("Travel rate ($/hr)").fill("150");

  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Saved" }).first()).toBeVisible();
  await expect(page.getByTestId("charges-total-estimate")).toHaveText("$1,050.00");
  if (inventoryToolsReady) {
    await expect(page.getByTestId("readiness-state")).toHaveText("Ready to send quote");
  } else {
    await expect(page.getByTestId("readiness-state")).toHaveText("1 checks remaining");
  }

  await page.reload();
  await expect(page.getByLabel("Labor hours")).toHaveValue("3");
  await expect(page.getByTestId("charges-total-estimate")).toHaveText("$1,050.00");

  await page.getByRole("link", { name: "Printed Estimate" }).click();
  await expect(page).toHaveURL(/\/estimates\/.+\/printed-estimate$/);
  await page.getByRole("button", { name: "Generate PDF" }).click();
  await expect(page.getByText("PDF generated", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Regenerate PDF" })).toBeVisible();

  await page.getByRole("link", { name: "Tasks List" }).click();
  await expect(page).toHaveURL(/\/estimates\/.+\/tasks$/);

  const taskTitle = `Phase5 task ${suffix}`;
  await page.getByTestId("new-task-title").fill(taskTitle);
  await page.getByRole("button", { name: "Add task" }).click();
  await expect(page.getByText(taskTitle)).toBeVisible();

  const taskToggle = page.getByLabel(`Mark ${taskTitle} complete`);
  await taskToggle.click();
  await expect(taskToggle).toBeChecked();

  await page.reload();
  await expect(page.getByText(taskTitle)).toBeVisible();
  await expect(page.getByLabel(`Mark ${taskTitle} complete`)).toBeChecked();
});
