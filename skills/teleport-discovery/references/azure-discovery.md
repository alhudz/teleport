# Azure Discovery

## Find `az` CLI

If `AZ` is already set, use it. Otherwise run `which az` silently — if successful, set `AZ=az`. If neither, stop:

> "The Azure CLI (`az`) is required. Install it from https://learn.microsoft.com/en-us/cli/azure/install-azure-cli"

Run `$AZ account show --query id --output tsv` silently. If not logged in, stop:

> "You're not logged in to Azure. Run `az login` and then run this skill again."

Set `SUBSCRIPTION_ID` from the output.

## Teleport Version Check

If `CLUSTER_VERSION` is below `18.8`, stop:

> "Azure Discovery requires Teleport 18.8 or later. Your cluster is running v<CLUSTER_VERSION>."

## Prerequisites

Inform the user before continuing:

1. Your Azure account needs permissions to create managed identities, role definitions, and role assignments in the target subscription(s).
2. Each VM to be discovered must have a managed identity assigned (system-assigned or user-assigned).
3. VMs must run a supported Linux distribution (Ubuntu, Debian, RHEL, Amazon Linux 2, or similar).

## Collect Configuration

Extract all values already provided in the prompt. For each missing required field, ask
conversationally. Present the menu to show current state; redisplay after each change.

**Menu** (redisplay after each change):

```
Azure Discovery Configuration

  Managed Identity
    Resource group: <value or "(required)">
    Location:       <value or "eastus (default)">

  Discovery Matchers
    Subscriptions:   <value or "(required)">
    Regions:         <value or "* (all)">
    Resource groups: <value or "* (all)">
    Tags:            <value or "* (all)">

  Terraform directory: <value or "./teleport-azure-discovery">

  Discovery group: cloud-discovery-group (fixed)   ← Teleport Cloud
                   <value or "(required)">         ← self-hosted

Say what you'd like to change, or "confirm" to proceed.
```

Render only one Discovery group line based on whether `PROXY_ADDR` ends in `.teleport.sh` or `.cloud.gravitational.io` (both are Teleport Cloud domains).

**Configuration** — skip any questions for fields already provided in the user's prompt.
Present all three questions together in a single `AskUserQuestion` call. You MUST include
all three — do not skip any.

```
Question 1 (title: "Managed Identity"):
  "Which existing resource group and location should the managed identity go in?"
  (free text, required — e.g. 'my-rg' or 'my-rg in westeurope'; location defaults to eastus)
  Do NOT offer to create resource groups — these must already exist in Azure.

Question 2 (title: "Azure Matchers"):
  "Which VMs should Teleport discover? (subscription <SUBSCRIPTION_ID> is pre-selected)"
  Options:
    - All VMs in this subscription
    - Match by region
    - Match by resource group
    - Match by tags (e.g. teleport-auto-enroll=true)
  Free-text placeholder: "Describe what to match, e.g. westus in resource group 'production'"

Question 3 (title: "Terraform"):
  "Where should I write the Terraform files?"
  Options:
    - Use current directory (<WORKDIR inferred from cwd or default ./teleport-azure-discovery>)
    - Use a different directory
  Free-text placeholder: "Enter a directory path"
```

Set `AZURE_MANAGED_IDENTITY_RESOURCE_GROUP` and `AZURE_MANAGED_IDENTITY_LOCATION` from Question 1.

For Question 2, follow up after the response if values are needed:
- **All VMs**: "Any additional subscription IDs to enroll? (comma-separated, or leave blank)"
- **Region**: "Which region(s)? e.g. `westus, eastus`" (if unsure: suggest `az account list-locations --output table`)
- **Resource group**: "Which resource group(s)?"
- **Tags**: "Which tag(s)? e.g. `env=prod, teleport-auto-enroll=true`"

If the user typed specific values directly (e.g. "westus"), apply them without a follow-up.

`subscriptions` always starts with `<SUBSCRIPTION_ID>`; append any extras. Validate that every
subscription ID is a well-formed Azure UUID (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`). If any
ID fails validation, tell the user and ask them to correct it before continuing.
→ HCL: `subscriptions = ["id1", "id2"]`

Parse into HCL fields. Omit fields not set:
→ `regions`, `resource_groups`, `tags` (tag values are lists)

Do not infer or suggest tags from existing Terraform files.

Set `WORKDIR` from Question 3. Detect whether `WORKDIR` contains an existing Terraform project and adapt accordingly:

- **No existing project** → create a complete, self-contained Terraform project (providers + module) ready to `terraform apply`.
- **Existing project** → integrate into it: find and update an existing module definition, or add a new one. Edit provider config as needed.

Use Grep to search for an existing module reference — scoped to `WORKDIR` only:

```
Grep: "terraform.releases.teleport.dev/teleport/discovery/azure"
path: <WORKDIR>
glob: "*.tf"
```

**If the module reference is found (existing discovery config):**

Read the matching file and extract current values:
- `teleport_proxy_public_addr`
- `azure_resource_group_name`
- `azure_managed_identity_location`
- `azure_matchers` (subscriptions, regions, resource_groups, tags)

Pre-populate the config menu with these values and go directly into the menu flow. Skip any `AskUserQuestion` for fields already resolved from the existing config or the user's prompt. Only ask for values that remain ambiguous.

After confirmation, use the `Edit` tool to update the module definition in the file where it was found.

**If `.tf` files exist but no module reference (existing project without discovery):**

Use the `Edit` tool to add any missing providers to the existing provider configuration file:

```hcl
terraform {
  required_providers {
    # Add if not already present:
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= <CLUSTER_VERSION>"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
  }
}

# Add if not already present:
provider "teleport" {
  addr = "<PROXY_ADDR>"
}

provider "azurerm" {
  features {}
}
```

Then use the `Write` tool to create `<WORKDIR>/azure_discovery.tf` with the module definition.

**If no `.tf` files (new project):**

Use the `Write` tool to create both files:
- `<WORKDIR>/versions.tf` — providers and versions
- `<WORKDIR>/azure_discovery.tf` — module definition and output

**Discovery group** — Cloud: `cloud-discovery-group` (fixed). Self-hosted: prompt for the value — must match `discovery_group` in the Discovery Service config. For private clusters see the [Azure VM Auto-Discovery (Terraform) docs](https://goteleport.com/docs/enroll-resources/auto-discovery/servers/azure-vm-discovery/azure-vm-discovery-terraform/).

**"confirm"** — require resource group and subscriptions before proceeding. Present summary:

> Install managed identity in resource group `<RG>` (location: `<LOCATION>`)
> Enroll subscription(s) `<SUBSCRIPTIONS>`
> [Match VMs by `<REGIONS>` / resource group `<RG_MATCHERS>` / tags `<TAG_MATCHERS>` — omit if not set]
> Write Terraform files to `<WORKDIR>`
>
> Does this look right, or would you like to change anything?

## Generate Terraform Files

Fill in all values from the configuration step and use the `Write` or `Edit` tool to propose
the files. The user will see a diff and can approve or reject each change. After writing,
print a configuration summary.

**`versions.tf` template:**

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= <CLUSTER_VERSION>"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
  }
}

provider "teleport" {
  addr = "<PROXY_ADDR>"
}

provider "azurerm" {
  features {}
}
```

**`azure_discovery.tf` template:**

```hcl
module "azure_discovery" {
  source  = "terraform.releases.teleport.dev/teleport/discovery/azure"
  version = "~> <MODULE_VERSION>"

  teleport_proxy_public_addr    = "<PROXY_ADDR>"
  teleport_discovery_group_name = "<DISCOVERY_GROUP>"

  azure_resource_group_name       = "<AZURE_MANAGED_IDENTITY_RESOURCE_GROUP>"
  azure_managed_identity_location = "<AZURE_MANAGED_IDENTITY_LOCATION>"

  azure_matchers = [
    {
      types         = ["vm"]
      subscriptions = [<SUBSCRIPTIONS — each ID quoted, comma-separated>]
      # regions         = [...] — include only if configured
      # resource_groups = [...] — include only if configured
      # tags            = {...} — include only if configured
    }
  ]
}

output "azure_discovery" {
  value = module.azure_discovery
}
```

After writing the files, print a configuration summary:

```
Configuration summary:
- Cluster proxy:                   <PROXY_ADDR>
- Subscriptions:                   <SUB_1>, <SUB_2>, ...
- Discovery group:                 <DISCOVERY_GROUP>
- Managed Identity Resource Group: <AZURE_MANAGED_IDENTITY_RESOURCE_GROUP>
- Managed Identity Location:       <AZURE_MANAGED_IDENTITY_LOCATION>
- Output directory:                <WORKDIR>
```

## Apply Terraform

Explain what each command does, then present them together:

> **You're ready to apply. Here's what each command does:**
>
> - `terraform init` — downloads the Teleport discovery module and Azure provider
> - `eval "$(tctl terraform env)" && terraform apply` — authenticates with short-lived credentials, then creates the managed identity, role definitions, and role assignments in Azure, and registers the OIDC integration with Teleport
>
> ```bash
> cd <WORKDIR>
> terraform init
> eval "$(tctl terraform env)" && terraform apply
> ```
>
> Run these when you're ready. Once applied, come back and I can verify that discovery is working.

## Verify

Run silently after the user confirms apply is complete:

```bash
$TCTL discovery nodes --cloud=azure
```

Lists each VM Teleport has attempted to enroll, with status (`Online`, `Installed (offline)`, `Failed`). Present results clearly. If no rows appear yet:

> "Discovery runs automatically every ~5 minutes. Check back soon with:
>
> ```
> tctl discovery nodes --cloud=azure
> ```"

Link to the web UI: `https://<PROXY_HOST>/web/integrations` — use the hostname from the proxy address, without the port (e.g. `example.teleport.sh:443` → `https://example.teleport.sh/web/integrations`)
