curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/api/v1/pools/1 \
  --insecure \
  --request DELETE | jq
