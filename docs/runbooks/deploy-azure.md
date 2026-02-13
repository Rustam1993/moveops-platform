# Runbook: Azure Deploy (Phase 7)

## Scope
- Azure Container Apps (web external, api internal)
- Azure Database for PostgreSQL Flexible Server
- OpenTofu state in existing Storage Account backend
- GitHub Actions OIDC (no long-lived Azure secrets)
- Migrations via Container Apps Job

## One-time setup checklist
Already provided in this repo:
- Subscription: `e8ee16b5-7cd9-4a48-8bd3-6322b6366e74`
- Tenant: `20b954a7-5f25-4e30-82e1-4b850848605a`
- Region: `eastus`
- Resource group: `rg-moveops-mvp`
- Existing ACR: `moveacr` (`moveacr.azurecr.io`)
- Existing state backend:
  - Storage account: `moveopsstate1993`
  - Container: `tfstate`
  - Key: `azure/mvp.tfstate`

You must ensure:
1. GitHub OIDC app exists and has federated credentials for this repo/workflows.
2. The OIDC principal has rights to deploy into `rg-moveops-mvp`.
3. The OIDC principal has `AcrPush` on the existing ACR (already granted per project context).
4. Dependency Graph is enabled in the repo (for Dependency Review action).

## GitHub Secrets (names only)
Azure auth:
- `AZURE_CLIENT_ID`
- `AZURE_TENANT_ID`
- `AZURE_SUBSCRIPTION_ID`

Terraform variables (passed as `TF_VAR_*`):
- `TF_VAR_pg_admin_user`
- `TF_VAR_pg_admin_password`
- `TF_VAR_pg_db_name`
- `TF_VAR_session_secret`
- `TF_VAR_csrf_secret`

## Deploy flow (CI/CD)
### 1) Build & push images
Workflow: `.github/workflows/build-and-push.yml`
- Trigger: push to `main` or manual dispatch
- Produces:
  - `moveacr.azurecr.io/moveops/api:<sha>`
  - `moveacr.azurecr.io/moveops/web:<sha>`

### 2) Apply IaC + run migrations
Workflow: `.github/workflows/deploy-azure.yml`
- Trigger: after successful Build & Push, or manual dispatch
- Steps:
  1. `tofu init/plan/apply` in `infra/iac/envs/azure/mvp`
  2. starts `moveops-db-migrate` Container Apps Job and waits for success
  3. smoke checks:
     - `curl $WEB_URL/login`
     - `curl $WEB_URL/api/health` (web-proxied API)
     - verify API internal FQDN is not publicly reachable from GitHub runner

## Rollback
- Re-deploy by dispatching `deploy-azure.yml` with a previous `image_tag`.
- If needed, scale down web/api revisions manually in Azure Portal or via `az containerapp`.

## Migrations (manual)
To run migrations job manually:
```bash
az containerapp job start -n moveops-db-migrate -g rg-moveops-mvp
az containerapp job execution list -n moveops-db-migrate -g rg-moveops-mvp -o table
```

## ACR pull access (required)
Container Apps pull images from the existing ACR using managed identity.
If image pulls fail, grant `AcrPull` to:
- web app identity
- api app identity
- db-migrate job identity

You can get principal IDs via `tofu output`:
```bash
cd infra/iac/envs/azure/mvp
tofu output container_app_web_principal_id
tofu output container_app_api_principal_id
tofu output container_app_job_migrate_principal_id
```
Then assign `AcrPull` manually (not managed in IaC by project rule).

## Deferred (intentionally)
- Key Vault
- Private networking (VNet + private endpoints)
- WAF/front door
- Customer-managed keys
