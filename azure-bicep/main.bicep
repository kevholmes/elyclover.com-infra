targetScope = 'resourceGroup' // set scope to resource group inside of existing subscription

@maxLength(24)
@minLength(2)
param media_storage_acct_name string = 'elyclovercommedia'
param location string = 'centralus'
param rg_name string = 'root'

// storage accounts, CDN endpoints in front of them
module stg './elyclovercom-media.bicep' = {
  name: 'storageDeployment'
  scope: resourceGroup(rg_name)
  params: {
    storageAccountName: media_storage_acct_name
    location: location
  }
}

// DNS CNAME records for ${env}.elyclover.com pointing at CDN endpoints
