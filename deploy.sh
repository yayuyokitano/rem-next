set -e

changeall=false
declare -A changes
while read p; do
  changes["${p%%:*}"]=true
  if [[ $p != */* ]];then
    changeall=true
    break
  fi
done < git-diff.txt

while read p; do
  IFS=' = ' read -r -a envArray <<< "$p"
  declare "${envArray[0]}=${envArray[1]}"
done < config.ini

for d in */ ; do
  [[ $d == _* ]] && continue 
  [[ ${changes[$d]} == true ]] && continue
  [[ $changeall == true ]] && continue
  cd "${d%/}"

  envList=""
  while read p; do
    envList="${envList}${p}=${!p},"
  done < env.txt
  while read p; do
    envList="${envList}${p}=$(gcloud secrets versions access latest --secret ${p}),"
  done < secretenv.txt

  gcloud functions deploy "${d%/}" --set-env-vars="${envList%,}" --vpc-connector rem-connector --region=us-central1 --source . --trigger-http --allow-unauthenticated --runtime go116
  cd ../
done