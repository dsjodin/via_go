curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/pools \
  --insecure \
  --request POST \
  --data '{ "name": "sfo-m01", "netmask": 24, "net_address": "172.16.60.0", "gateway": "172.16.60.1", "only_serve_reimage": true, "lease_time": 3600 }' | jq

  curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/pools \
  --insecure \
  --request POST \
  --data '{ "name": "sfo-w01", "netmask": 24, "net_address": "172.16.61.0", "gateway": "172.16.61.1", "only_serve_reimage": true, "lease_time": 3600  }' | jq

  curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/pools \
  --insecure \
  --request POST \
  --data '{ "name": "sfo-efi", "netmask": 24, "net_address": "172.16.4.0", "gateway": "172.16.4.1", "only_serve_reimage": true, "lease_time": 3600 }' | jq