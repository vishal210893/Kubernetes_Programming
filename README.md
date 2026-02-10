# Kubernetes Custom Resource Client

This project demonstrates how to create a Kubernetes Custom Resource Definition (CRD) and a client application to interact with custom resources.

## 📚 Documentation

This project includes comprehensive documentation about **runtime.Object** implementation in Kubernetes:

**→ [START HERE](START_HERE.md)** - Choose your learning path and find what you need

### Quick Links
- **[Detailed Guide](pkg/apis/cnat/v1alpha1/RUNTIME_OBJECT_EXPLAINED.md)** - Deep understanding
- **[Visual Flows](pkg/apis/cnat/v1alpha1/RUNTIME_OBJECT_FLOW.md)** - Diagrams and flows

All code files include comprehensive inline comments explaining the "why" behind runtime.Object.

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

## Using the Makefile

This project includes a comprehensive Makefile for common operations. View all available targets:

```bash
make help
```

### Common Commands

**Development:**
```bash
make build           # Build the client binary
make run             # Run the application
make clean           # Clean build artifacts
make fmt             # Format code
make vet             # Run go vet
make test            # Run tests
make vendor          # Vendor dependencies
```

**Code Generation:**
```bash
make manifests       # Generate CRD manifests
make generate        # Generate client code (deepcopy, clientset, listers, informers)
make codegen         # Run all code generation (manifests + client code)
```

**Kubernetes Operations:**
```bash
make install-crd     # Install CRD to cluster
make apply-cr        # Apply custom resources
make get-ats         # List all At resources
make describe-crd    # Describe the CRD
make uninstall-crd   # Remove CRD from cluster
make delete-cr       # Delete custom resources
```

**Workflow Shortcuts:**
```bash
make all             # Clean, vendor, generate code, and build
make deploy          # Generate manifests, install CRD, and apply CR
make undeploy        # Delete CR and uninstall CRD
make refresh         # Refresh deployment (undeploy then deploy)
make setup           # Initial setup: vendor dependencies and check tools
```

**Git Operations:**
```bash
make git-status                        # Show git status
make git-commit MSG="your message"     # Commit changes
make git-push                          # Push to remote
make git-sync MSG="your message"       # Commit and push
```

## Code Generation

This project uses Kubernetes code-generator to generate:
- DeepCopy methods
- Typed clientset
- Listers
- Informers

### Manual Code Generation

To regenerate the client code after modifying the API types:

```bash
./hack/update-codegen.sh
```

Or use the Makefile:

```bash
make generate
```

To regenerate CRD manifests after modifying kubebuilder markers:

```bash
make manifests
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

