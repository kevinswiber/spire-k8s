openssl x509 -req -days 360 -in skbridge.csr -CA ./skbridge.ca.crt  -CAkey ./skbridge.ca.key -CAcreateserial -out skbridge.crt -extfile ./v3ext.cnf -extensions v3_req
