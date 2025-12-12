curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/groups \
  --insecure \
  --request GET  | jq
