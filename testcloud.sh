set -e

#get the environment variables for tests
while read p; do
  IFS=' = ' read -r -a envArray <<< "$p"
  declare "${envArray[0]}=${envArray[1]}"
  export ${envArray[0]}
done < config.ini

for d in */ ; do
  [[ $d == __* ]] && continue
  cd "${d%/}"
  go test
  cd ../
done