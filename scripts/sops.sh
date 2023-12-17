#!/bin/bash -x

# encrypt or decrypt all tls assets at rest in src control
# see project root's .sops.yaml for SOPS defaults

TLS_FILE_LOCATION=./assets/tls
DEC_POSTFIX="dec"
ENC_POSTFIX="enc"

SOPS_SUCCESS="SOPS operation successful"
SOPS_FAILURE="SOPS operation failed, bailing out before destroying any files"

if [ -z "$1" ]; then
 echo "requires either 'encrypt or 'decrypt' argument e.g.: ./sops.sh encrypt"
fi

if [ "$1" == "encrypt" ]; then
  FILES=("$TLS_FILE_LOCATION"/*".$DEC_POSTFIX")
  for f in "${FILES[@]}"
  do
    newName=${f//dec/enc}
    echo encrypting "$f" to "$newName"
    if sops --output "$newName" --encrypt "$f"
    then
      echo "$SOPS_SUCCESS"
      rm "$f"
    else
      echo "$SOPS_FAILURE"
      exit 1
    fi
  done
elif [ "$1" == "decrypt" ]; then
  FILES=("$TLS_FILE_LOCATION"/*.pfx".$ENC_POSTFIX")
  for f in "${FILES[@]}"
  do
    newName=${f//enc/dec}
    echo decrypting "$f" to "$newName"
    if sops --output "$newName" --decrypt "$f"
    then
      echo "$SOPS_SUCCESS"
      rm "$f"
    else
      echo "$SOPS_FAILURE"
      exit 1
    fi
  done
else
  echo "FATAL: unknown argument: $1"
fi
