set -e

#get the environment variables for tests
while read p; do
  IFS=' = ' read -r -a envArray <<< "$p"
  declare "${envArray[0]}=${envArray[1]}"
  export ${envArray[0]}
done < config.ini

while read p; do
  declare "$p=$(gcloud secrets versions access latest --secret $p)"
  export $p
done < secretenv.txt

cd verify-user
go test
