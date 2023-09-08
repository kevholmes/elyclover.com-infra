param storageAccountName string
param location string
param containerName string = 'public-media'
param environmentNames array = [
  'dev'
  'staging'
  'prod'
]

output storageAccountName string = storageAccountName
output containerName string = containerName

////// Storage Accounts to host media assets behind CDN //////

resource storageAccount_resource 'Microsoft.Storage/storageAccounts@2022-09-01' = [for name in environmentNames: {
  name: '${storageAccountName}${name}'
  location: location
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    allowBlobPublicAccess: true
    networkAcls: {
      bypass: 'AzureServices'
      virtualNetworkRules: []
      ipRules: []
      defaultAction: 'Allow'
    }
    supportsHttpsTrafficOnly: true
    encryption: {
      services: {
        blob: {
          keyType: 'Account'
          enabled: true
        }
      }
      keySource: 'Microsoft.Storage'
    }
    accessTier: 'Hot'
  }
}]

resource blobService_resource 'Microsoft.Storage/storageAccounts/blobServices@2022-09-01' = [for i in range (0, length(environmentNames)): {
  name: 'default'
  parent: storageAccount_resource[i]
  properties: {
    cors: {
      corsRules: []
    }
    deleteRetentionPolicy: {
      enabled: false
    }
  }
}]

resource storageContainer_resources 'Microsoft.Storage/storageAccounts/blobServices/containers@2022-09-01' = [for i in range(0, length(environmentNames)): {
  parent: blobService_resource[i]
  name: containerName
  properties: {
    immutableStorageWithVersioning: {
      enabled: false
    }
    defaultEncryptionScope: '$account-encryption-key'
    denyEncryptionScopeOverride: false
    publicAccess: 'Blob'
  }
}]

////// CDN that sits in front of Storage Accounts //////

var endpointName = 'endpoint-elyclovercom-${uniqueString(resourceGroup().id)}'
var profileName = 'cdn-elyclovercom-${uniqueString(resourceGroup().id)}'
//var storageAccountHostName = replace(replace(storageAccount_resource.properties.primaryEndpoints.blob, 'https://', ''), '/', '')

resource cdnProfile 'Microsoft.Cdn/profiles@2021-06-01' = {
  name: profileName
  location: location
  tags: {
    displayName: profileName
  }
  sku: {
    name: 'Standard_Microsoft'
  }
}

resource endpoint 'Microsoft.Cdn/profiles/endpoints@2021-06-01' = [for (name, i) in environmentNames: {
  parent: cdnProfile
  name: '${endpointName}-${name}'
  location: location
  tags: {
    displayName: endpointName
  }
  properties: {
    originHostHeader: replace(replace(storageAccount_resource[i].properties.primaryEndpoints.blob, 'https://', ''), '/', '')
    isHttpAllowed: false
    isHttpsAllowed: true
    queryStringCachingBehavior: 'IgnoreQueryString'
    contentTypesToCompress: [
      'text/plain'
      'text/html'
      'text/css'
      'application/x-javascript'
      'text/javascript'
    ]
    isCompressionEnabled: true
    origins: [
      {
        name: 'origin1'
        properties: {
          hostName: replace(replace(storageAccount_resource[i].properties.primaryEndpoints.blob, 'https://', ''), '/', '')
        }
      }
    ]
  }
}]

//output hostName string = endpoint.properties.hostName
//output originHostHeader string = endpoint.properties.originHostHeader
