openssl x509 -req -days 360 -in spire-agent.csr -CA ~/.minikube/ca.crt  -CAkey ~/.minikube/ca.key -CAcreateserial -out spire-agent.crt 
