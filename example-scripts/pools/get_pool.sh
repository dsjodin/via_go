curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/pools \
  --insecure \
  --request GET | jq
