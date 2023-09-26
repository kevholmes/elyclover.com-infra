#!/bin/bash
# convert a pem cert to pfx type as Azure Key Vault would prefer them
INKEY=$1
INCRT=$2
OUTPFX=$3

echo "ex: ./pem-to-pfx.sh yourkey.key yourcrt.crt outpfx.pfx"

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then
  echo "missing args, re-read instructions"
  exit 1
else
  openssl pkcs12 -inkey "${1}" -in "${2}" -export -out "${3}"
fi
