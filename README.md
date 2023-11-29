# function-cidrsubnet

A [composition function][functions] in [Go][go].

`function-cidrsubnet` allows you to generate a new subnet CIDR within a parent prefix.

The function takes three inputs:
- `prefix`: the parent prefix in CIDR notation
- `newbits`: the number of bits to extend the prefix by to create the new subnet
- `netnum`: the value to insert into the extended bits

The calculated CIDR will be stored in the function pipline context under a key equal to the `metadata.name` of the function Input.

For example, an Input like
```yaml
- step: generate-subnet
    functionRef:
      name: function-cidrsubnet
    input:
      apiVersion: cidrsubnet.fn.crossplane.io/v1beta1
      kind: Input
      metadata:
        name: subnet-a
      prefix: 10.10.0.0/24
      newbits: 4
      netnum: 15
```
Will insert the value `10.10.0.240/28` into the context key `subnet-a`

The input fields also accept lookup references that use the Patch and Transform [FieldPath syntax][field selectors]. You can lookup values from the Function request Observed state, Desired state, or Context.  
```yaml
- step: generate-subnet
    functionRef:
      name: function-cidrsubnet
    input:
      apiVersion: cidrsubnet.fn.crossplane.io/v1beta1
      kind: Input
      metadata:
        name: subnet-a
      prefix: ${observed.resources[vpc].status.atProvider.cidrBlock}
      newbits: 4
      netnum: 15
```


## Building Locally

This function is built using [Go][go], [Docker][docker], and the [Crossplane CLI][cli], and was generated from the [function template][function template]

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --quiet --platform=linux/amd64 --tag runtime-amd64
$ docker build . --quiet --platform=linux/arm64 --tag runtime-arm64

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build --package-root=package --embed-runtime-image=runtime-amd64 --package-file=function-amd64.xpkg
$ crossplane xpkg build --package-root=package --embed-runtime-image=runtime-arm64 --package-file=function-arm64.xpkg

# Push the function image
$ crossplane xpkg push --package-files=function-amd64.xpkg,function-arm64.xpkg {{registry}}/function-cidrsubnet:{{version}}
```

[functions]: https://docs.crossplane.io/latest/concepts/composition-functions
[go]: https://go.dev
[function guide]: https://docs.crossplane.io/knowledge-base/guides/write-a-composition-function-in-go
[function template]: https://github.com/crossplane/function-template-go
[package docs]: https://pkg.go.dev/github.com/crossplane/function-sdk-go
[docker]: https://www.docker.com
[cli]: https://docs.crossplane.io/latest/cli%  
[field selectors]: https://docs.crossplane.io/latest/concepts/patch-and-transform/#selecting-fields