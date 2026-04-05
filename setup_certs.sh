#!/bin/bash

cwd=`pwd`

ch_dir() {
    local server_name=$1
    cd $cwd
    rm -rf certs/${server_name}
    mkdir -p certs/${server_name}
    cd certs/${server_name}
}

generate_pem() {
    local server_name=$1
    local name=$2
    local password=${server_name}12345
    
    openssl req -newkey rsa:2048 -keyout ${name}.key -out ${name}.csr -passin pass:${password} -passout pass:${password} -subj "/CN=${name}/OU=TEST/O=${name} Test/L=Oslo/C=NO"

    #convert the key to PKCS8, otherwise kafka/java cannot read it
    openssl pkcs8 -topk8 -in ${name}.key -inform pem -v1 PBE-SHA1-RC4-128 -out ${name}-pkcs8.key -outform pem -passin pass:${password} -passout pass:${password}

    mv ${name}-pkcs8.key ${name}.key

    # Sign the CSR with the root CA
    # do not indent following unindented text block
    openssl x509 -req -CA root.crt -CAkey root.key -in ${name}.csr -out ${name}.crt -sha256 -days 365 -CAcreateserial -passin pass:${password} -extensions v3_req -extfile <(cat <<EOF
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no
[req_distinguished_name]
CN = ${name}
[v3_req]
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${name}
DNS.2 = localhost
EOF
)

    openssl rsa -in ${name}.key -out ${name}-unencrypted.key -passin pass:${password}

    # Combine private key and cert in one file
    cat ${name}.key ${name}.crt > ${name}.pem

}

generate_kafka() {
    local server_name="kafka"
    local password=${server_name}12345

    echo -e "\n================================Generating certs for ${server_name}================================"
    ch_dir ${server_name}
    
    openssl req -x509 -days 365 -newkey rsa:2048 -keyout root.key -out root.crt -subj "/CN=Certificate Authority/OU=TEST/O=${server_name} Test/L=Oslo/C=NO" -passin pass:$password -passout pass:$password

    for i in broker client
    do
        echo "Create a certificate for $i"
        generate_pem $server_name  $i
    done
}

generate_redis() {
    local server_name="redis"
    local password=${server_name}12345

    echo -e "\n================================Generating certs for ${server_name}================================"
    ch_dir ${server_name}
    
    openssl req -x509 -days 365 -newkey rsa:2048 -keyout root.key -out root.crt -subj "/CN=Certificate Authority/OU=TEST/O=${server_name} Test/L=Oslo/C=NO" -passin pass:$password -passout pass:$password

    for i in server client
    do
        echo "Create a certificate for $i"
        generate_pem $server_name  $i
    done
}

generate_redis
generate_kafka
