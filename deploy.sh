set -e

while read p; do
  IFS=' = ' read -r -a envArray <<< "$p"
  declare "${envArray[0]}=${envArray[1]}"
done < config.ini

for d in */ ; do
  [[ $d == _* ]] && continue 
  cd "${d%/}"

  envList=""
  while read p; do
    envList="${envList}${p}=${!p},"
  done < env.txt
  while read p; do
    envList="${envList}${p}=$(gcloud secrets versions access latest --secret ${p}),"
  done < secretenv.txt

  gcloud functions deploy "${d%/}" --set-env-vars="${envList%,}"  --region=us-central1 --source . --trigger-http --runtime go116
  cd ../
done