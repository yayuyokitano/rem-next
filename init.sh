set -e

#get the environment variables
while read p; do
  IFS=' = ' read -r -a envArray <<< "$p"
  declare "${envArray[0]}=${envArray[1]}"
  export ${envArray[0]}
done < config.ini

for d in */ ; do
  [[ $d != __* ]] && continue
  cd "${d%/}"

  #get secrets
  while read p; do
    declare "${p}=$(gcloud secrets versions access latest --secret ${p})"
    export ${p}
  done < secretenv.txt

  go run .
  cd ../
done