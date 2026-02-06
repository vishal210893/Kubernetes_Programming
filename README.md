# Kubernetes Custom Resource Client

This project demonstrates how to create a Kubernetes Custom Resource Definition (CRD) and a client application to interact with custom resources.

## Custom Resource: At

The `At` custom resource allows you to schedule commands to run at specific times.

### Structure

```yaml
apiVersion: cnat.programming-kubernetes.info/v1alpha1
kind: At
metadata:
  name: example-at
spec:
  schedule: "2019-07-03T02:00:00Z"  # UTC timestamp
  command: "echo YAY"                # Command to execute
status:
  phase: "PENDING"                   # Optional: PENDING or DONE
```

### Additional Printer Columns

The CRD is configured to display helpful information when using `kubectl get ats`:

```bash
kubectl get ats
```

Output:
```
NAME           SCHEDULE               COMMAND                 PHASE     AGE
example-at     2019-07-03T02:00:00Z   echo YAY                          10m
example-at-2   2026-02-08T10:30:00Z   echo Hello Kubernetes   PENDING   2m
```

The following columns are displayed:
- **Schedule**: The scheduled time for command execution
- **Command**: The command to be executed
- **Phase**: Current execution phase (PENDING/DONE)
- **Age**: Time since the resource was created

### Categories

The `At` resource is categorized under `atsall` for easier resource management and discovery.

## Setup

### 1. Apply the CRD

```bash
kubectl apply -f hack/crd.yaml
```

### 2. Create a Custom Resource

```bash
kubectl apply -f hack/cr.yaml
```

## Build and Run

### Build the client

```bash
go build -o bin/at-client pkg/main.go
```

### Run the client

List At resources in the default namespace:
```bash
./bin/at-client
```

List At resources in a specific namespace:
```bash
./bin/at-client -namespace my-namespace
```

Specify a custom kubeconfig:
```bash
./bin/at-client -kubeconfig /path/to/kubeconfig
```

## Code Generation

This project uses Kubernetes code-generator to generate:
- DeepCopy methods
- Typed clientset
- Listers
- Informers

To regenerate the client code after modifying the API types:

```bash
./hack/update-codegen.sh
```

## Project Structure

```
.
├── hack/
│   ├── boilerplate.go.txt      # License header for generated code
│   ├── crd.yaml                # CustomResourceDefinition
│   ├── cr.yaml                 # Sample custom resource
│   └── update-codegen.sh       # Code generation script
├── pkg/
│   ├── apis/
│   │   └── cnat/
│   │       ├── group.go        # Group definition
│   │       └── v1alpha1/
│   │           ├── doc.go      # Package documentation
│   │           ├── types.go    # At resource type definitions
│   │           ├── register.go # Scheme registration
│   │           └── zz_generated.deepcopy.go  # Generated
│   ├── generated/              # All generated client code
│   │   ├── clientset/
│   │   ├── informers/
│   │   └── listers/
│   └── main.go                 # Client application
├── tools.go                    # Build-time dependencies
└── go.mod
```

## How It Works

1. **CRD Definition**: `hack/crd.yaml` defines the structure of the `At` custom resource
2. **API Types**: `pkg/apis/cnat/v1alpha1/types.go` contains Go struct definitions
3. **Code Generation**: The `update-codegen.sh` script generates client code
4. **Client Application**: `pkg/main.go` uses the generated clientset to interact with the cluster

## Key Improvements

- ✅ Fixed deprecated `List()` method - now uses `List(ctx, metav1.ListOptions{})`
- ✅ Clean code structure with proper error handling
- ✅ Configurable namespace via command-line flag
- ✅ Better formatted output
- ✅ Updated CRD from v1beta1 to v1

