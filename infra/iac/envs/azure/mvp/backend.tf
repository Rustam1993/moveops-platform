terraform {
  backend "azurerm" {
    resource_group_name  = "rg-moveops-mvp"
    storage_account_name = "moveopsstate1993"
    container_name       = "tfstate"
    key                  = "azure/mvp.tfstate"

    # Use Azure AD / GitHub OIDC (no storage keys).
    use_azuread_auth = true
    use_oidc         = true

    subscription_id = "e8ee16b5-7cd9-4a48-8bd3-6322b6366e74"
    tenant_id       = "20b954a7-5f25-4e30-82e1-4b850848605a"
    client_id       = "296afa81-0d9a-47f5-aeb5-96df465be15f"
  }
}
