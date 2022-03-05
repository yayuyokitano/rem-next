set -e

changeall=0
declare -A changes
while read p; do
  if [[ $p != */* ]];then
    changeall=1
    break
  fi
  changes[${p%%/*}]=1
done < git-diff.txt

while read p; do
  IFS=' = ' read -r -a envArray <<< "$p"
  declare "${envArray[0]}=${envArray[1]}"
done < config.ini

for d in */ ; do
  [[ $d == _* ]] && continue 
  ([[ ${changes[${d%/}]} != 1 || $changeall != 1 ]]) && continue
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