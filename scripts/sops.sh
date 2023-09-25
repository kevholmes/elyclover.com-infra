#!/bin/bash

TLS_FILE_LOCATION=./assets/tls
FILES=("$TLS_FILE_LOCATION"/*)

if [ -z "$1" ]; then
 echo "requires either 'encrypt or 'decrypt' argument e.g.: ./sops.sh encrypt"
fi

if [ "$1" == "encrypt" ]; then
  for f in "${FILES[@]}"
  do
    echo encrypting "$f"
    sops --encrypt --in-place "$f"
  done
elif [ "$1" == "decrypt" ]; then
  for f in "${FILES[@]}"
  do
    echo decrypting "$f"
    sops --decrypt --in-place "$f"
  done
else
  echo "FATAL: unknown argument: $1"
fi
