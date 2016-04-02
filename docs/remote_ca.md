# Remote CA support

The document shows an example of using a remote Certficiate Authority for
managing your certificates.

**WARNING: This example first demonstrates setting things up insecurely, then refines the model with mutual TLS.


## Initial CA setup, insecure mode first

This example uses the CFSSL package.  You can build it from scratch, or leverage
the existing public UCP image which bundles CFSSL.

```sh
VOLUME=cluster-root-ca
IMAGE=docker/ucp-cfssl:1.0.4

docker pull ${IMAGE}

# Cleanup and prior cruft to start clean
docker volume rm ${VOLUME}
docker volume create --name ${VOLUME}

# Set up the configuration file(s)
docker run --entrypoint sh -i -v ${VOLUME}:/etc/cfssl ${IMAGE} -c "cat - > /etc/cfssl/cluster_root_CA.json" << EOF
{
    "key": {
        "algo": "rsa",
        "size": 4096
    },
    "CN": "Cluster Root CA"
}
EOF

docker run --entrypoint sh -i -v ${VOLUME}:/etc/cfssl ${IMAGE} -c "cat - > /etc/cfssl/cluster_config.json" << EOF
{ "signing": { "default": { "expiry": "8760h" }, "profiles": {
            "client": { "usages": [
                            "signing",
                            "key encipherment",
                            "client auth"
                    ], "expiry": "8760h" },
            "node": { "usages": [
                            "signing",
                            "key encipherment",
                            "server auth",
                            "client auth"
                    ], "expiry": "8760h" }
} } }
EOF


# Generate root
docker run -v ${VOLUME}:/etc/cfssl ${IMAGE} genkey -initca cluster_root_CA.json | docker run --entrypoint cfssljson -i -v ${VOLUME}:/etc/cfssl ${IMAGE} -bare cluster_root_CA
```

Now you can run the server **WARNING THIS IS INSECURE - See below for instructions showing how to add mutual TLS**

```sh
# Now run an insecure server
docker run -p 8888:8888 -v ${VOLUME}:/etc/cfssl ${IMAGE} \
    serve -address 0.0.0.0 \
        -ca /etc/cfssl/cluster_root_CA.pem \
        -ca-key /etc/cfssl/cluster_root_CA-key.pem \
        -config /etc/cfssl/cluster_config.json
```


Once you have the CA running, you can now create machines where the certs are signed by this CA.

## Generating your initial Client setup

You can use the standard location for storing certs
`$HOME/.docker/machine/certs` however if you have already run machine,
you should remove any existing data there, or use the new `--cert-path`
flag to machine to use an alternative location.  Note: You can wire up
different sets of machines to different root CAs by using different
`--cert-paths` for them. (typically also specifying a different
`--storage-path` to keep the inventories separate.

Example invocation:
```sh
docker-machine -D --ca http://localhost:8888 \
    --cert-path ${HOME}/.docker/machine/certs2 \
    create --driver kvm d2
```

This will detect that you don't already have a ca.pem, and retrieve it
from the CA.  It will also detect you do not have a client cert, and
will get one signed by the CA.  It will then create the VM, and during
the creation, generate a CSR for the machine and get it signed by the
remote CA.

## Now make it Secure with Mutual TLS

* Kill the CA you started above

* Then generate a server cert for the CA to use, signed by the same root CA. **Make sure to replace the IP/hostnames below for your environment!**
```sh
docker run --entrypoint sh -i -v ${VOLUME}:/etc/cfssl ${IMAGE} -c "cat - > /etc/cfssl/cluster_CA_server.json" << EOF
{
    "hosts": [
        "127.0.0.1",
        "localhost"
    ],
    "key": {
        "algo": "rsa",
        "size": 4096
    },
    "CN": "My CA Server"
}
EOF
docker run -v ${VOLUME}:/etc/cfssl ${IMAGE} gencert -config=cluster_config.json -ca cluster_root_CA.pem -ca-key cluster_root_CA-key.pem -profile=node cluster_CA_server.json | docker run --entrypoint cfssljson -i -v ${VOLUME}:/etc/cfssl ${IMAGE} -bare server
```

* Generate a client so you can connect to it (you already have one if you've been following along, but here's how to generate more)
```sh
docker run --entrypoint sh -i -v ${VOLUME}:/etc/cfssl ${IMAGE} -c "cat - > /etc/cfssl/client.json" << EOF
{
    "key": {
        "algo": "rsa",
        "size": 4096
    },
    "CN": "My Client Cert"
}
EOF
docker run -v ${VOLUME}:/etc/cfssl ${IMAGE} gencert -config=cluster_config.json -ca cluster_root_CA.pem -ca-key cluster_root_CA-key.pem -profile=client client.json | docker run --entrypoint cfssljson -i -v ${VOLUME}:/etc/cfssl ${IMAGE} -bare client
docker run --entrypoint sh -v ${VOLUME}:/etc/cfssl ${IMAGE} -c "mkdir client; cp cluster_root_CA.pem client/ca.pem && mv client.pem client/cert.pem && mv client-key.pem client/key.pem && tar cf - client" | tar xvf -
```
* Now you've got a set of client certs in `./client/`
* Start up the CA server in mutual TLS mode

```sh
docker run -p 8888:8888 -v ${VOLUME}:/etc/cfssl ${IMAGE} \
    serve -address 0.0.0.0 \
        -ca /etc/cfssl/cluster_root_CA.pem \
        -ca-key /etc/cfssl/cluster_root_CA-key.pem \
        -config /etc/cfssl/cluster_config.json \
        -tls-key /etc/cfssl/server-key.pem \
        -tls-cert /etc/cfssl/server.pem \
        -mutual-tls-ca /etc/cfssl/cluster_root_CA.pem
```

* Do a quick "curl" test to make sure you can connect to it with your new certs:
```sh
curl -s \
    --cert client/cert.pem \
    --key client/key.pem \
    --cacert client/ca.pem \
https://localhost:8888/api/v1/cfssl/info
```
* That endpoint requires a payload, so it'll give an error, but if you get something like the following, then you've got it working:
```
{"success":false,"result":null,"errors":[{"code":405,"message":"Method is not allowed:\"GET\""}],"messages":[]}
```

* You can do a quick negative test of machine to confirm random people can't sign stuff:
```sh
% docker-machine --ca https://localhost:8888 --cert-path ${HOME}/.docker/machine/certs3 create --driver kvm d1

Error getting cert signed by remote CA: Failed to query info from remote CA: Post https://localhost:8888/api/v1/cfssl/info: x509: certificate signed by unknown authority
```
* Now try again and replace the `--cert-path` so it points to where you just generated those client certs and it should work

```sh
% docker-machine --ca https://localhost:8888 --cert-path ${HOME}/.docker/machine/certs3 create --driver kvm d2
```

* And now you can connect to the new machine using any certs signed by that CA!

