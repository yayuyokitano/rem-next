set -e

changeall=0
declare -A changes
while read p; do
  if [[ $p != */* ]];then
    if [[ $p != secretenv.txt && $p != config.ini ]];then
      continue
    fi
    changeall=1
    break
  fi
  changes[${p%%/*}]=1
done < /workspace/git-diff.txt

env=""
while read p; do
  env+="$p=$(gcloud secrets versions access latest --secret "$p"),"
done < secretenv.txt

while read p; do
  IFS=' = ' read -r -a envArray <<< "$p"
  env+="${envArray[0]}=${envArray[1]},"
done < config.ini

for d in */ ; do
  [[ $d == _* ]] && continue
  [[ ${changes[${d%/}]} != 1 && $changeall != 1 ]] && continue
  cd "${d%/}"

  gcloud functions deploy "${d%/}" --set-env-vars="${env%,}" --vpc-connector rem-connector --region=us-central1 --source . --trigger-http --allow-unauthenticated --runtime go116
  cd ../
done