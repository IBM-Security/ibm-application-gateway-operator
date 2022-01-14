# Introduction
The IBM Application Gateway can be deployed in Kubernetes alongside an application using the sidecar pattern. This allows an application developer with limited or no understanding of the IBM Application Gateway configuration to add a predefined set of annotations to the application deployment YAML to create the required sidecar container.

![Sidecar Deployment Model](images/operator-sidecar.png)

> Note: It is assumed that the IBM Application Gateway operator has already been installed into the environment and is available.

# Steps

The following steps will:

* Create an application that will be fronted by IBM Application Gateway
* Access the application via the IBM Application Gateway
* The page will show the identity and other headers that have been added to the request by the IBM Application Gateway

> This example does not include authentication. This example illustrates the deployment process for using IBM Application Gateway as a proxy to a protected application. 

1. Paste the following into a file named test\_config.yaml.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  config: |
    version: "21.12"
    resource_servers:
      - path: /demo-local
        connection_type: tcp
        servers:
        - host: localhost
          port: 8000
        identity_headers:
          attributes:
          - attribute: AZN_CRED_AUTHNMECH_INFO
            header: mech_info
          - attribute: AZN_CRED_REGISTRY_ID
          jwt:
            certificate: |
              -----BEGIN CERTIFICATE-----
              MIIDSzCCAjOgAwIBAgIUFeqbJIDD0ITJN12OXKLNR9FaLaYwDQYJKoZIhvcNAQEL
              BQAwNTELMAkGA1UEBhMCdXMxDDAKBgNVBAoMA2libTEYMBYGA1UEAwwPd3d3Lmlh
              Z2RlbW8uY29tMB4XDTIwMDUxNTAwMDQyOVoXDTIxMDUxNTAwMDQyOVowNTELMAkG
              A1UEBhMCdXMxDDAKBgNVBAoMA2libTEYMBYGA1UEAwwPd3d3LmlhZ2RlbW8uY29t
              MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAssbA0VB0WTikufBOVaUC
              tIDyVQ9tknM7yxVg+pJc7zA/nVWvNbZ+sV6lVi1E9wPcKDvH1dd9N9mAOK9E1/KK
              Nma8+o5Q10k1OSDaPR50aHY6iukFOJiGaM0L3FVsmJO6i0UXZ1ZztkPHM3xEE6rm
              BYvgBYCeHSmMNWrdh/U8Pnte2dKn5GImOLzyK/CeZJpiE/HXsDGY68BedAQSClZn
              TMQNpM6Rynmbn7tRrCf6aUDhFnmrcHea3WldI5dJzPe2UD0k3EBdO/S/yA6bEZ5/
              sz9gW0hTQsLGlrnLDJbH7AtcBiK7iltDNHCBfghBIYRBxr/kxx+/MtjfCWlKWKwU
              /wIDAQABo1MwUTAdBgNVHQ4EFgQUjqw+fFV1XHm21/jwANQ407RhVogwHwYDVR0j
              BBgwFoAUjqw+fFV1XHm21/jwANQ407RhVogwDwYDVR0TAQH/BAUwAwEB/zANBgkq
              hkiG9w0BAQsFAAOCAQEAsa4AjCqBspPlZdNhGLpX+jigKJ9klV7XZAi5C7CbM1uj
              SjztbNIbycxQsGjduJgxzhXCUUkPZWsOqb5/5j+/PrSGWRpDc1Lct34wDWYP2gHJ
              cszQ+J1JTkPTs7/ZEnutcWpl3WH72w/Rp156fSK6Xi+D7mbOn2ubv92d4YO3aXHE
              3r6BDx824lhQ+BQgReWBsMmx8VG8vDu52LlRIUAGbIPENmgG3LqU36NVdqAzAlTI
              bRsHxPALQWZlX/VZvng66vJEXPhY1en4BztN8Lfxp+MGHykQOCZBton0bndIjF4b
              A+lSMYUkbYS7WwCgWKxF3FF2d/XGHCkjPUKmf5CmLg==
              -----END CERTIFICATE-----
              -----BEGIN PRIVATE KEY-----
              MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCyxsDRUHRZOKS5
              8E5VpQK0gPJVD22SczvLFWD6klzvMD+dVa81tn6xXqVWLUT3A9woO8fV13032YA4
              r0TX8oo2Zrz6jlDXSTU5INo9HnRodjqK6QU4mIZozQvcVWyYk7qLRRdnVnO2Q8cz
              fEQTquYFi+AFgJ4dKYw1at2H9Tw+e17Z0qfkYiY4vPIr8J5kmmIT8dewMZjrwF50
              BBIKVmdMxA2kzpHKeZufu1GsJ/ppQOEWeatwd5rdaV0jl0nM97ZQPSTcQF079L/I
              DpsRnn+zP2BbSFNCwsaWucsMlsfsC1wGIruKW0M0cIF+CEEhhEHGv+THH78y2N8J
              aUpYrBT/AgMBAAECggEAGs6IrRo2SAahVKw+4sK6dBx9udl1YadbyOjswMXfn5IJ
              h+sIObKirq/1dMWW+L8gbPwBzAPi59P6UQWe+g6JQP0Iyz05y/5TN7zAXLfLVrqb
              rqagPMtKJD7Sz3gTr5E/QJDcoIcrFKJ3JTY18wkGMsC1l72gd9j6CCx9CVzxaqDO
              0U1OzmZphKLkneIzCK+f644QmeK6sA1nOjpeudoK7o/Trm9/vBxZ2QuNjjw3LuOB
              U5DdDwoWiD5ovcQBgHm4Y6XlAJkxDZtIYG2RU8GbICbMqc3nWqu11Q22+lCUAX5z
              /twHBK8wxu3DpJNIVzUc+roIWb4W6BsJwO5W/OJjOQKBgQDm23jeBxdp7jpbwsOW
              kNyhbgRTPteI3h+Il4CdR+ExIzCjAZqOPA6lQmcXwMMdUB6eXzPIwyYAi/8IJ4Wt
              JX4HuPbAOCEj68jqvGphF9CH7zq1PFdEpqGjDQeUej9ZZMfroDrGNcpahzoV8yov
              HNVDBEB9hJA8S/qyaWx890NBdQKBgQDGPzayCSEpJ9NxN18uJpCx823QGGFQ3316
              MtseKU/cTEFtwbrZBpZdJnSyH3L/j318BopN53XFtbZFJCrZGvPCTTtHUj1Y+FHQ
              hStbkp6cXDVPVB9RpjxFQOwffW81KbQYcV4nRuj5ZeXUmJJyjKFW+LV03UOb2nFx
              RPAeIbtaIwKBgQCZpYSiMRISPK22mL/YG/3bqv9R8Ec/HqbaIAtHoqlsLMGYG98j
              vzD4KGO42WCCx+o4+Z/bhJZsUsCgSr2baZKhzSbc8/6Ki8kjAdfDWzg84KmwKcLj
              PAvkGy3qUhqLgpnUZsX3WaGd8tZj+X0f8g62FcReRbZBi0gFKkk8Os0WaQKBgDzH
              0sAg9m7EzBnxDRh8MG6uoKNlZijCbJkTkISdy+NR6dl5xQyW7XDfwnV+GObrmmB1
              LOAXr88PGDP4k75+ee+/eAY026Q0dfkFazanLEPqLVoGA5gn7GCbOAmrjnXxNEsu
              LtzB69nS+PUGVhgSr8CXfiK3V+wnvBIrPacvVl6bAoGBALsM7g2E064AKg1v7IJt
              QtCg0ILmMu7+5KjmEPqXPOwhaRYN4Nw91b+EGjGbLKEeCABoitJ+Bpj68pKtkw19
              2D2qmPlb2qdGCUYZUDLSbliveI2q0byQcMvoonHbZaQ3UfLzRzaSNQxOUgU+BXGJ
              v40vQ1XaeKlVtXuzhsmJ4gZw
              -----END PRIVATE KEY-----
            claims:
            - attr: acr
            - attr: sub
            hdr_name: jwt
```

2. Create the IBM Application Gateway configuration configmap.

```shell
kubectl apply -f test_config.yaml
```

3. Paste the following into a file named deployment.yaml.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  labels:
    app: test
    name: test
  annotations:
    ibm-application-gateway.security.ibm.com/env.LANG: en
    ibm-application-gateway.security.ibm.com/configuration.test.type: configmap
    ibm-application-gateway.security.ibm.com/configuration.test.name: test-config
    ibm-application-gateway.security.ibm.com/configuration.test.dataKey: config
    ibm-application-gateway.security.ibm.com/configuration.test.order: "1"
    ibm-application-gateway.security.ibm.com/deployment.image: ibmcom/ibm-application-gateway:21.12.0
    ibm-application-gateway.security.ibm.com/deployment.imagePullPolicy: IfNotPresent
    ibm-application-gateway.security.ibm.com/service.port: "30441"
spec:
  selector:
    matchLabels:
      app: test
      name: test
  replicas: 1
  template:
    metadata:
      labels:
        app: test
        name: test
    spec:
      containers:
      - name: test
        image: ibmcom/ibm-application-gateway-demo-resource-server:21.12.0
        imagePullPolicy: IfNotPresent
```

4. Create the deployment.

```shell
kubectl apply -f deployment.yaml
```

5. Check the deployment. There should be a deployment application named “test” and a pod with a prefix of “test” that has 2 running containers.

```shell
kubectl get all

NAME                                                    READY   STATUS    RESTARTS   AGE
pod/ibm-application-gateway-operator-5b55989d98-w22n9   1/1     Running   0          12m
pod/test-7797447ccf-zg7nq                               2/2     Running   0          34s

NAME                                                    TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
service/ibm-application-gateway-injector-webhook-svc    ClusterIP   10.105.52.14    <none>        443/TCP             10m
service/ibm-application-gateway-operator-metrics        ClusterIP   10.97.147.42    <none>        8383/TCP,8686/TCP   12m
service/kubernetes                                      ClusterIP   10.96.0.1       <none>        443/TCP             32m
service/test-ibm-application-gateway-sidecar-svc2tmd4   NodePort    10.111.179.97   <none>        8443:30441/TCP      34s

NAME                                               READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/ibm-application-gateway-operator   1/1     1            1           12m
deployment.apps/test                               1/1     1            1           34s

NAME                                                          DESIRED   CURRENT   READY   AGE
replicaset.apps/ibm-application-gateway-operator-5b55989d98   1         1         1       12m
replicaset.apps/test-7797447ccf                               1         1         1       34s
```

6. Expose the IBM Application Gateway port.

```shell
kubectl port-forward deployment.apps/iag-instance 30441:30441
```

7. Leave the console as is and using a browser access the demo URL. Make sure its https rather than http.

For example:

```
https://127.0.0.1:30441/demo-local
```

8. The demo page is shown. 

![Demo Application](images/intro-generic-demoapp.png)

> This page includes the following pieces that have been added by the IBM Application Gateway: <br/>
>    - A JWT header has been added <br/>
>    - The AZN-CRED-REGISTRY-ID header has been added <br/>
>    - The MECH-INFO header has been added <br/>
>    - The JWT has been extracted.

9. Exit the port-forwarding service by pressing ctrl-c in the shell.

10. Delete the test application.

```shell
kubectl delete -f deployment.yaml
```

11. Check the deployment and service are removed.

12. Cleanup.

```shell
kubectl delete -f test_config.yaml
```
