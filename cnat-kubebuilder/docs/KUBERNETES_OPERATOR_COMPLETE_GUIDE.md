# Kubernetes Operator Development: Complete Guide

> A comprehensive guide covering Operators, Controllers, Reconciliation, Kubebuilder, controller-runtime, and client-go.

---

## Table of Contents

1. [Operator vs Controller](#1-operator-vs-controller)
2. [The Reconcile Function](#2-the-reconcile-function)
3. [reconcile.Result Explained](#3-reconcileresult-explained)
4. [When Does Reconcile Run?](#4-when-does-reconcile-run)
5. [State Machine Pattern](#5-state-machine-pattern)
6. [The Architecture Stack](#6-the-architecture-stack)
7. [Kubebuilder + controller-runtime](#7-kubebuilder--controller-runtime)
8. [client-go vs controller-runtime](#8-client-go-vs-controller-runtime)
9. [Spring Boot Analogy](#9-spring-boot-analogy)
10. [Complete Code Walkthrough](#10-complete-code-walkthrough)
11. [Best Practices](#11-best-practices)

---

## 1. Operator vs Controller

### What is a Controller?

A **Controller** is a control loop that:
- Watches the state of resources in Kubernetes
- Compares **desired state** (spec) with **actual state** (status)
- Takes actions to move actual state toward desired state

**Built-in Kubernetes Controllers:**
- Deployment Controller
- ReplicaSet Controller
- Job Controller
- Service Controller

### What is an Operator?

An **Operator** is:
- A **Controller** + **Custom Resource Definitions (CRDs)**
- Extends Kubernetes API with domain-specific knowledge
- Automates operational tasks (installation, upgrades, backups, etc.)
- Encodes human operator knowledge into software

### Key Difference

```
┌─────────────────────────────────────────────────────────────┐
│  Controller = Watches BUILT-IN Kubernetes resources         │
│               (Pods, Services, Deployments, etc.)           │
├─────────────────────────────────────────────────────────────┤
│  Operator   = Controller + CRD                              │
│               (Watches CUSTOM resources you define)         │
└─────────────────────────────────────────────────────────────┘
```

### Example: Your `At` Operator

Your project is an **Operator** because:

1. **Custom Resource Definition (CRD)**: `At` resource
2. **Controller**: `AtReconciler` watches `At` resources
3. **Domain Knowledge**: Schedules commands at specific times
4. **State Machine**: PENDING → RUNNING → DONE

```yaml
# Your Custom Resource
apiVersion: cnat.programming-kubernetes.info/v1alpha1
kind: At
metadata:
  name: example-at
spec:
  schedule: "2026-02-10T20:30:00Z"
  command: "echo Hello Kubernetes"
status:
  phase: PENDING
```

### Analogy

| Concept | Analogy |
|---------|---------|
| **Controller** | Traffic light that manages flow |
| **Operator** | Smart traffic system that learns patterns and adapts (controller + intelligence) |

---

## 2. The Reconcile Function

### What is Reconciliation?

**Reconciliation** is the heart of the Kubernetes controller pattern. It's a control loop that continuously works to make the **actual state** match the **desired state**.

```
┌─────────────────────────────────────────────────────────────┐
│              THE RECONCILIATION LOOP                        │
│                                                             │
│    ┌──────────┐      ┌────────────┐      ┌──────────┐      │
│    │ Desired  │ ───▶ │ Reconcile  │ ───▶ │  Actual  │      │
│    │  State   │      │  Function  │      │  State   │      │
│    │  (Spec)  │      │            │      │ (Status) │      │
│    └──────────┘      └────────────┘      └──────────┘      │
│         ▲                  │                   │           │
│         │                  │                   │           │
│         └──────────────────┴───────────────────┘           │
│               Keep comparing and adjusting                  │
└─────────────────────────────────────────────────────────────┘
```

### The Reconcile Function Signature

```go
func (r *AtReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
```

**Parameters:**
- `ctx context.Context` - Context for cancellation and timeouts
- `req ctrl.Request` - Contains namespace and name of the resource to reconcile

**Returns:**
- `ctrl.Result` - Tells Kubernetes WHEN to call Reconcile again
- `error` - If non-nil, triggers retry with exponential backoff

### The Reconcile Principle

```
┌────────────────────────────────────────────────────────────┐
│  INPUT:  Request (namespace + name of changed resource)    │
│  OUTPUT: (Result, error) - tells k8s WHEN to reconcile     │
│  GOAL:   Make actual state match desired state             │
└────────────────────────────────────────────────────────────┘
```

### Key Characteristics

1. **Event-Driven**: Not a polling loop - triggered by events
2. **Idempotent**: Can be called multiple times safely
3. **Level-Triggered**: Works on current state, not events
4. **Single Resource**: One reconcile call = one resource

---

## 3. reconcile.Result Explained

The `reconcile.Result` struct controls **when** Kubernetes will call your `Reconcile()` function again.

```go
type Result struct {
    Requeue      bool          // Should we requeue immediately?
    RequeueAfter time.Duration // Requeue after this duration
}
```

### All Return Value Combinations

#### 1. `return reconcile.Result{}, nil` - Success, Don't Requeue

```go
// Use when: Resource is stable, nothing more to do
case cnatv1alpha1.PhaseDone:
    return reconcile.Result{}, nil
```

**Behavior:**
- ✅ No error
- ⏸️ Won't requeue automatically
- 🔔 Will only run again if:
  - Someone edits the resource
  - An owned resource changes

---

#### 2. `return reconcile.Result{}, err` - Error, Retry with Backoff

```go
// Use when: Transient error occurred
d, err := timeUntilSchedule(instance.Spec.Schedule)
if err != nil {
    return reconcile.Result{}, err
}
```

**Behavior:**
- ❌ Error occurred
- 🔄 Requeues with **exponential backoff**:
  - 1st retry: ~1 second
  - 2nd retry: ~2 seconds
  - 3rd retry: ~4 seconds
  - ... up to 5 minutes max
- 📝 Error is logged

---

#### 3. `return reconcile.Result{RequeueAfter: duration}, nil` - Schedule for Later

```go
// Use when: Need to check again after specific time
if d > 0 {
    return reconcile.Result{RequeueAfter: d}, nil
}
```

**Behavior:**
- ✅ No error
- ⏰ Requeues after **exact duration**
- 🎯 Perfect for time-based operations
- 💡 More efficient than polling!

---

#### 4. `return reconcile.Result{Requeue: true}, nil` - Requeue Immediately

```go
// Use when: Need immediate retry without error (rare)
return reconcile.Result{Requeue: true}, nil
```

**Behavior:**
- ✅ No error
- 🔄 Requeues immediately
- ⚠️ Use sparingly (can cause tight loops)

---

#### 5. `return reconcile.Result{RequeueAfter: duration}, err` - Error Wins

```go
// Error takes precedence over RequeueAfter
return reconcile.Result{RequeueAfter: 1*time.Hour}, fmt.Errorf("oops")
```

**Behavior:**
- ❌ Error occurred
- 🔄 Uses **error backoff** (ignores RequeueAfter)

---

### Quick Reference Table

| Return Value | Behavior | Use Case |
|-------------|----------|----------|
| `Result{}, nil` | Success, wait for events | Resource is stable |
| `Result{}, err` | Error, retry with backoff | Transient failures |
| `Result{RequeueAfter: d}, nil` | Success, requeue after duration | Time-based operations |
| `Result{Requeue: true}, nil` | Success, requeue immediately | Force immediate retry (rare) |
| `Result{RequeueAfter: d}, err` | Error wins, uses backoff | Error takes precedence |

---

## 4. When Does Reconcile Run?

The `Reconcile()` function is called automatically by controller-runtime when:

### 4.1 Resource Events (Create/Update/Delete)

```yaml
# When you run: kubectl apply -f at.yaml
apiVersion: cnat.programming-kubernetes.info/v1alpha1
kind: At
metadata:
  name: example-at
spec:
  schedule: "2026-02-10T20:30:00Z"
  command: "echo Hello"
```

→ **Reconcile is triggered** with `req.Name = "example-at"`

### 4.2 Owned Resource Changes

When a Pod (owned by your At resource) changes:
- Pod starts running → Reconcile called
- Pod succeeds/fails → Reconcile called
- Pod deleted → Reconcile called

This works because of `SetupWithManager`:

```go
func (r *AtReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&cnatv1alpha1.At{}).        // Watch At resources
        Owns(&corev1.Pod{}).             // Watch Pods owned by At
        Named("at").
        Complete(r)
}
```

### 4.3 RequeueAfter Timer

When you return `Result{RequeueAfter: 5*time.Minute}`:
→ Kubernetes waits 5 minutes, then calls Reconcile again

### 4.4 Error Backoff

When you return an error:
→ Kubernetes retries with exponential backoff (1s, 2s, 4s, 8s, ...)

### Event Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│  1. Kubernetes API Server                                   │
│     (Resource events: Create/Update/Delete)                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  2. Informer/Cache (controller-runtime)                     │
│     - Watches API server for changes                        │
│     - Caches resources locally                              │
│     - Detects changes                                       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  3. Work Queue (controller-runtime)                         │
│     - Queues reconcile requests                             │
│     - Handles rate limiting                                 │
│     - Manages retries                                       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  4. Controller Manager (controller-runtime)                 │
│     - Dequeues requests                                     │
│     - Calls YOUR Reconcile() function                       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  5. YOUR AtReconciler.Reconcile()                           │
│     - Your business logic                                   │
│     - Returns Result{}                                      │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  6. Back to Work Queue                                      │
│     - Based on Result{}, requeue or done                    │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. State Machine Pattern

Your controller implements a **State Machine** pattern:

```
┌─────────────┐
│   PENDING   │ ◄─── Resource created
└──────┬──────┘
       │
       │ Time not reached?
       │ ├─ YES → return Result{RequeueAfter: duration}, nil
       │ └─ NO  → Move to RUNNING
       │
       ▼
┌─────────────┐
│   RUNNING   │
└──────┬──────┘
       │
       │ Pod exists?
       │ ├─ NO  → Create Pod, return Result{}, nil
       │ ├─ YES (running) → return Result{}, nil (wait for Pod event)
       │ └─ YES (done)    → Move to DONE
       │
       ▼
┌─────────────┐
│    DONE     │ ◄─── Pod succeeded/failed
└──────┬──────┘
       │
       │ return Result{}, nil (finished)
       ▼
     (end)
```

### Phase Transitions in Code

```go
switch instance.Status.Phase {
case cnatv1alpha1.PhasePending:
    // Check if scheduled time has arrived
    d, err := timeUntilSchedule(instance.Spec.Schedule)
    if d > 0 {
        // Not yet time - schedule reconcile for later
        return reconcile.Result{RequeueAfter: d}, nil
    }
    // Time arrived! Transition to RUNNING
    instance.Status.Phase = cnatv1alpha1.PhaseRunning

case cnatv1alpha1.PhaseRunning:
    // Create or check Pod
    if podFinished {
        instance.Status.Phase = cnatv1alpha1.PhaseDone
    }

case cnatv1alpha1.PhaseDone:
    // Nothing to do
    return reconcile.Result{}, nil
}

// Update status after phase transitions
r.Status().Update(ctx, instance)
```

---

## 6. The Architecture Stack

### Who Calls Reconcile?

**You never call Reconcile() yourself!** controller-runtime does it for you.

```
┌─────────────────────────────────────────────────────────────┐
│  You NEVER write:                                           │
│  reconciler.Reconcile(ctx, req)  // ❌ DON'T DO THIS        │
├─────────────────────────────────────────────────────────────┤
│  controller-runtime does it internally:                     │
│                                                             │
│  for {                                                      │
│      req := workQueue.Get()                                 │
│      result, err := reconciler.Reconcile(ctx, req)          │
│                                                             │
│      if err != nil {                                        │
│          workQueue.AddRateLimited(req) // Retry with backoff│
│      } else if result.Requeue {                             │
│          workQueue.Add(req)            // Requeue immediately│
│      } else if result.RequeueAfter > 0 {                    │
│          workQueue.AddAfter(req, result.RequeueAfter)       │
│      }                                                      │
│      // else: done, wait for events                         │
│  }                                                          │
└─────────────────────────────────────────────────────────────┘
```

### The Complete Stack

```
┌─────────────────────────────────────────────────────────────┐
│  Kubebuilder (CLI Tool)                                     │
│  - Generates project structure                              │
│  - Creates boilerplate code                                 │
│  - NOT a runtime library                                    │
└────────────────────────┬────────────────────────────────────┘
                         │ generates code that uses
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  controller-runtime (Runtime Library)                       │
│  - sigs.k8s.io/controller-runtime                           │
│  - Runs the event loop                                      │
│  - Calls your Reconcile()                                   │
│  - Manages watches, caches, queues                          │
└────────────────────────┬────────────────────────────────────┘
                         │ built on top of
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  client-go (Kubernetes Client Library)                      │
│  - k8s.io/client-go                                         │
│  - Talks to Kubernetes API                                  │
│  - Provides informers, listers, workqueues                  │
└────────────────────────┬────────────────────────────────────┘
                         │ talks to
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  Kubernetes API Server                                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. Kubebuilder + controller-runtime

### What is Kubebuilder?

**Kubebuilder** is a CLI scaffolding tool that:
- Generates project structure
- Creates boilerplate code
- Uses controller-runtime by default
- Is **NOT** a runtime library (not in your `go.mod`)

### What is controller-runtime?

**controller-runtime** is a runtime framework that:
- Runs the event loop
- Manages informers, caches, work queues
- Calls your `Reconcile()` function
- Is **IN** your `go.mod` as a dependency

### Domain Name in Kubebuilder

When using Kubebuilder, the **domain name is appended to the group name**:

```bash
kubebuilder init --domain programming-kubernetes.info
kubebuilder create api --group cnat --version v1alpha1 --kind At
```

**Results in:**
- **Full API Group**: `cnat.programming-kubernetes.info`
- **API Version**: `v1alpha1`
- **Full GVK**: `cnat.programming-kubernetes.info/v1alpha1, Kind=At`

**In your CRD YAML:**
```yaml
apiVersion: cnat.programming-kubernetes.info/v1alpha1
kind: At
```

### What Kubebuilder Generates

```bash
kubebuilder init --domain programming-kubernetes.info
kubebuilder create api --group cnat --version v1alpha1 --kind At
```

**Creates:**
```
cnat-kubebuilder/
├── cmd/
│   └── main.go                    # ← Manager setup (ctrl.NewManager)
├── api/v1alpha1/
│   ├── at_types.go                # ← CRD definition (spec/status)
│   └── groupversion_info.go       # ← GVK registration
├── internal/controller/
│   └── at_controller.go           # ← YOUR Reconcile() logic
├── config/
│   ├── crd/                       # ← Generated CRD YAML
│   ├── rbac/                      # ← Generated RBAC rules
│   └── manager/                   # ← Deployment configs
├── Dockerfile
├── Makefile
└── go.mod                         # ← Dependencies (controller-runtime!)
```

### Where is the Manager Code?

The manager setup is in `cmd/main.go`:

```go
func main() {
    // Create the manager (uses controller-runtime)
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme:                 scheme,
        Metrics:                metricsserver.Options{BindAddress: metricsAddr},
        HealthProbeBindAddress: probeAddr,
        LeaderElection:         enableLeaderElection,
    })

    // Register your reconciler
    if err = (&controller.AtReconciler{
        Client: mgr.GetClient(),
        Scheme: mgr.GetScheme(),
    }).SetupWithManager(mgr); err != nil {
        os.Exit(1)
    }

    // Start the event loop
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        os.Exit(1)
    }
}
```

---

## 8. client-go vs controller-runtime

### Can You Use Both Together?

**YES!** They're compatible and often used together:
- **controller-runtime wraps client-go** - it doesn't replace it
- You can use both in the same project

### Three Possible Combinations

#### 1. Pure client-go (Your `controller.go`)

```go
// YOU manually build everything
import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/informers"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
)

type Controller struct {
    kubeClientset kubernetes.Interface
    atLister      listers.AtLister
    workqueue     workqueue.RateLimitingInterface
}

// Manual informer setup
atInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: controller.enqueueAt,
    UpdateFunc: func(old, new interface{}) { controller.enqueueAt(new) },
})

// Manual worker loops
for i := 0; i < threadiness; i++ {
    go wait.Until(c.runWorker, time.Second, stopCh)
}
```

**Lines of code:** ~400  
**Boilerplate:** 80%

---

#### 2. controller-runtime with Kubebuilder (Your `at_controller.go`)

```go
// controller-runtime handles everything
import (
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

type AtReconciler struct {
    client.Client  // ← Wraps client-go internally
}

// Just write business logic
func (r *AtReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Your logic here
    return ctrl.Result{}, nil
}

// Wire up automatically
func (r *AtReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&cnatv1alpha1.At{}).
        Owns(&corev1.Pod{}).
        Complete(r)
}
```

**Lines of code:** ~150  
**Boilerplate:** 10%

---

#### 3. Both Together (Mixed Approach)

```go
import (
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    
    // Also import client-go for specialized features
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/record"
)

type AtReconciler struct {
    client.Client                          // ← controller-runtime
    KubeClientset kubernetes.Interface     // ← client-go
    Recorder      record.EventRecorder     // ← client-go
}

func (r *AtReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Use controller-runtime for basic operations
    r.Get(ctx, req.NamespacedName, instance)
    r.Create(ctx, pod)
    
    // Use client-go for specialized features (events)
    r.Recorder.Event(instance, corev1.EventTypeNormal, "Created", "Pod created")
    
    return ctrl.Result{}, nil
}
```

### When to Use Each

| **Use controller-runtime for:** | **Use client-go directly for:** |
|--------------------------------|--------------------------------|
| Reconcile loop framework | Events (better control) |
| CRD operations (Get/Create/Update) | Discovery API |
| Watch setup (For/Owns) | Exec into pods |
| Unstructured/dynamic clients | Port forwarding |
| Standard CRUD operations | Legacy code integration |

### Comparison Table

| **Aspect** | **client-go** | **controller-runtime** |
|------------|---------------|------------------------|
| **Informers** | `AddEventHandler(...)` | `For(&At{}).Owns(&Pod{})` ✨ |
| **Listers** | `atLister.Ats(ns).Get(name)` | `r.Get(ctx, name, &at)` ✨ |
| **Work Queue** | Manual setup | Hidden inside Manager ✨ |
| **Event Handlers** | Manual registration | Automatic ✨ |
| **Workers** | Manual goroutines | Automatic ✨ |
| **Requeue Logic** | `workqueue.AddAfter()` | `return Result{RequeueAfter: d}` ✨ |
| **Lines of Code** | ~400 | ~150 ✨ |

---

## 9. Spring Boot Analogy

### The Perfect Comparison

Your understanding is exactly right:

> "It's kind of similar to Spring Boot project instead of using Java Servlet directly"

```
┌─────────────────────────────────────────────────────────────┐
│  Java Web Development                                       │
│                                                             │
│  Java Servlets (Low-level)  →  Spring Boot (High-level)    │
│  - Manual HTTP handling        - Auto-configuration        │
│  - web.xml configuration       - @RestController           │
│  - Thread management           - Embedded Tomcat           │
├─────────────────────────────────────────────────────────────┤
│  Kubernetes Controller Development                          │
│                                                             │
│  client-go (Low-level)      →  controller-runtime          │
│  - Manual informers            - Auto-watches              │
│  - Manual work queues          - Reconcile() method        │
│  - Manual event handlers       - Embedded Manager          │
└─────────────────────────────────────────────────────────────┘
```

### Side-by-Side Code Comparison

#### Low-Level Approach

**Java Servlets:**
```java
public class UserServlet extends HttpServlet {
    @Override
    protected void doGet(HttpServletRequest req, HttpServletResponse resp) {
        // Manual parsing
        String userId = req.getParameter("id");
        
        // Manual business logic
        User user = userService.findById(userId);
        
        // Manual response building
        resp.setContentType("application/json");
        PrintWriter out = resp.getWriter();
        out.write(toJson(user));
        out.close();
    }
}

// Plus web.xml configuration...
```

**client-go:**
```go
type Controller struct {
    kubeClientset kubernetes.Interface
    atLister      listers.AtLister
    workqueue     workqueue.RateLimitingInterface
}

func (c *Controller) Run(stopCh <-chan struct{}) {
    // Manual informer setup
    atInformer.Informer().AddEventHandler(...)
    
    // Manual workers
    for i := 0; i < threadiness; i++ {
        go wait.Until(c.runWorker, time.Second, stopCh)
    }
}

func (c *Controller) processNextWorkItem() bool {
    // Manual queue processing
    obj, shutdown := c.workqueue.Get()
    defer c.workqueue.Done(obj)
    // ...
}
```

---

#### High-Level Framework

**Spring Boot:**
```java
@RestController
@RequestMapping("/users")
public class UserController {
    
    @Autowired
    private UserService userService;
    
    // Just write business logic!
    @GetMapping("/{id}")
    public User getUser(@PathVariable String id) {
        return userService.findById(id);  // That's it!
    }
}
```

**Kubebuilder + controller-runtime:**
```go
type AtReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// Just write business logic!
func (r *AtReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    instance := &cnatv1alpha1.At{}
    r.Get(ctx, req.NamespacedName, instance)  // That's it!
    
    // Your state machine logic here
    return ctrl.Result{}, nil
}
```

### The Exact Parallels

| **Aspect** | **Spring Boot** | **Kubebuilder + controller-runtime** |
|------------|----------------|-------------------------------------|
| **Low-level API** | Java Servlets | client-go |
| **High-level Framework** | Spring Boot | controller-runtime |
| **CLI Generator** | Spring Initializr | Kubebuilder |
| **Annotations** | `@RestController`, `@GetMapping` | `// +kubebuilder:rbac`, `For()`, `Owns()` |
| **Auto-config** | `@SpringBootApplication` | `ctrl.NewManager()` |
| **Embedded Server** | Tomcat/Jetty | Manager (event loop) |
| **You Write** | Business logic in controller methods | Business logic in `Reconcile()` |
| **Framework Handles** | HTTP, routing, serialization, threads | Events, caching, queuing, watches |
| **Config File** | `application.properties` | `config/` YAML manifests |
| **Request Object** | `HttpServletRequest` | `ctrl.Request` |
| **Response Object** | `ResponseEntity<T>` | `ctrl.Result` |

### Project Structure Comparison

**Spring Boot:**
```
my-rest-api/
├── src/main/java/com/example/
│   ├── Application.java           # ← @SpringBootApplication
│   ├── controller/
│   │   └── UserController.java    # ← @RestController (YOU write)
│   ├── service/
│   │   └── UserService.java       # ← Business logic
│   └── model/
│       └── User.java              # ← Data model
├── src/main/resources/
│   └── application.properties     # ← Config
└── pom.xml                        # ← Dependencies
```

**Kubebuilder:**
```
cnat-kubebuilder/
├── cmd/
│   └── main.go                    # ← ctrl.NewManager
├── internal/controller/
│   └── at_controller.go           # ← Reconciler (YOU write)
├── api/v1alpha1/
│   └── at_types.go                # ← Data model (CRD)
├── config/                        # ← Config (YAML)
└── go.mod                         # ← Dependencies
```

### The Philosophy

**Spring Boot Philosophy:**
> "You focus on **business logic** (REST endpoints),  
> we handle **plumbing** (HTTP, threads, serialization)"

**Kubebuilder Philosophy:**
> "You focus on **business logic** (reconciliation),  
> we handle **plumbing** (events, caching, queuing)"

---

## 10. Complete Code Walkthrough

### Your At Controller Explained

```go
// AtReconciler reconciles a At object
type AtReconciler struct {
    client.Client              // ← controller-runtime's unified client
    Scheme *runtime.Scheme     // ← For setting owner references
}
```

### RBAC Annotations

```go
// +kubebuilder:rbac:groups=cnat.programming-kubernetes.info,resources=ats,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cnat.programming-kubernetes.info,resources=ats/status,verbs=get;update;patch
```

These generate RBAC rules in `config/rbac/role.yaml` automatically!

### The Reconcile Flow

```go
func (r *AtReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // STEP 1: Fetch the resource
    instance := &cnatv1alpha1.At{}
    err := r.Get(ctx, req.NamespacedName, instance)
    if errors.IsNotFound(err) {
        return reconcile.Result{}, nil  // Resource deleted, nothing to do
    }

    // STEP 2: Initialize phase if empty
    if instance.Status.Phase == "" {
        instance.Status.Phase = cnatv1alpha1.PhasePending
    }

    // STEP 3: State machine
    switch instance.Status.Phase {
    case PhasePending:
        // Calculate time until schedule
        d, _ := timeUntilSchedule(instance.Spec.Schedule)
        if d > 0 {
            // Schedule is in the future - requeue for later
            return reconcile.Result{RequeueAfter: d}, nil
        }
        // Time arrived! Transition to RUNNING
        instance.Status.Phase = PhaseRunning

    case PhaseRunning:
        // Create Pod if not exists
        pod := newPodForCR(instance)
        controllerutil.SetControllerReference(instance, pod, r.Scheme)
        
        found := &corev1.Pod{}
        err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
        
        if errors.IsNotFound(err) {
            r.Create(ctx, pod)  // Create the pod
        } else if found.Status.Phase == corev1.PodSucceeded {
            instance.Status.Phase = PhaseDone  // Transition to DONE
        }

    case PhaseDone:
        return reconcile.Result{}, nil  // Finished!
    }

    // STEP 4: Update status
    r.Status().Update(ctx, instance)
    return reconcile.Result{}, nil
}
```

### SetupWithManager

```go
func (r *AtReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&cnatv1alpha1.At{}).    // Watch At resources
        Owns(&corev1.Pod{}).         // Watch Pods owned by At (add this!)
        Named("at").
        Complete(r)
}
```

**What this does automatically:**
- Creates **Informer** for `At` resources
- Creates **Informer** for `Pod` resources (filtered by owner)
- Sets up **event handlers** (Add/Update/Delete → enqueue)
- Creates **work queue** with rate limiting
- Starts **worker goroutines**

---

## 11. Best Practices

### 1. Reconcile Must Be Idempotent

```go
// ✅ Good: Check before creating
found := &corev1.Pod{}
err := r.Get(ctx, nsName, found)
if errors.IsNotFound(err) {
    r.Create(ctx, pod)  // Only create if not exists
}

// ❌ Bad: Create without checking
r.Create(ctx, pod)  // May fail if already exists
```

### 2. Use RequeueAfter Instead of Sleep

```go
// ✅ Good: Let controller-runtime schedule it
if timeUntil > 0 {
    return reconcile.Result{RequeueAfter: timeUntil}, nil
}

// ❌ Bad: Blocks the controller!
time.Sleep(5 * time.Minute)
```

### 3. Set Owner References

```go
// ✅ Good: Enables garbage collection
controllerutil.SetControllerReference(instance, pod, r.Scheme)

// When At is deleted → Pod is automatically deleted
```

### 4. Use Status Subresource

```go
// ✅ Good: Update status separately
r.Status().Update(ctx, instance)

// This prevents conflicts with spec updates
```

### 5. Return Errors for Transient Failures

```go
// ✅ Good: Let controller-runtime handle retries
if err != nil {
    return reconcile.Result{}, err  // Auto retry with backoff
}

// ❌ Bad: Swallow errors
if err != nil {
    log.Error(err, "something failed")
    return reconcile.Result{}, nil  // Won't retry!
}
```

### 6. Add Logging for Debugging

```go
reqLogger := log.FromContext(ctx).WithValues("namespace", req.Namespace, "at", req.Name)
reqLogger.Info("=== Reconciling At")
reqLogger.Info("Phase", "current", instance.Status.Phase)
```

### 7. Watch Owned Resources

```go
// Add Owns() to automatically reconcile when pods change
func (r *AtReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&cnatv1alpha1.At{}).
        Owns(&corev1.Pod{}).      // ← Important!
        Complete(r)
}
```

---

## Summary

### The Big Picture

```
┌─────────────────────────────────────────────────────────────┐
│  YOU: Write Business Logic                                  │
│  - Reconcile() method                                       │
│  - State transitions                                        │
│  - Resource creation                                        │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  Kubebuilder: Generates Scaffolding                         │
│  - Project structure                                        │
│  - Boilerplate code                                         │
│  - Makefiles, Dockerfiles                                   │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  controller-runtime: Runs Your Code                         │
│  - Event loop                                               │
│  - Informers, queues, watches                               │
│  - Calls Reconcile()                                        │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  client-go: Talks to Kubernetes                             │
│  - API client                                               │
│  - Informers implementation                                 │
│  - Watch/List operations                                    │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  Kubernetes API Server                                      │
└─────────────────────────────────────────────────────────────┘
```

### Key Takeaways

| **Concept** | **Summary** |
|-------------|-------------|
| **Operator** | Controller + CRD (extends Kubernetes) |
| **Controller** | Watches resources, reconciles state |
| **Reconcile()** | Your business logic, called by framework |
| **Result{}** | Controls when Reconcile runs again |
| **Kubebuilder** | CLI tool that scaffolds projects |
| **controller-runtime** | Framework that runs reconcilers |
| **client-go** | Low-level Kubernetes client library |

### The Evolution

```
2014: client-go released
  ↓
2018: controller-runtime created (to simplify client-go)
  ↓  
2018: Kubebuilder created (to scaffold controller-runtime projects)
  ↓
Today: Most operators use Kubebuilder + controller-runtime + client-go
```

### The Philosophy (Like Spring Boot!)

> **Pure client-go is great for learning internals,  
> but controller-runtime + Kubebuilder is the production-standard  
> approach for operator development in 2025!**

---

## Quick Reference Card

```
┌─────────────────────────────────────────────────────────────┐
│  RECONCILE RETURN VALUES                                    │
├─────────────────────────────────────────────────────────────┤
│  Result{}, nil              → Success, wait for events      │
│  Result{}, err              → Error, retry with backoff     │
│  Result{RequeueAfter: d}    → Success, check again in d     │
│  Result{Requeue: true}      → Success, requeue immediately  │
├─────────────────────────────────────────────────────────────┤
│  STATE MACHINE                                              │
├─────────────────────────────────────────────────────────────┤
│  PENDING  → (wait for time) → RUNNING                       │
│  RUNNING  → (create pod)    → DONE                          │
│  DONE     → (finished)                                      │
├─────────────────────────────────────────────────────────────┤
│  ARCHITECTURE                                               │
├─────────────────────────────────────────────────────────────┤
│  Kubebuilder        = Scaffolding (CLI tool)                │
│  controller-runtime = Framework (runs reconciler)           │
│  client-go          = Foundation (talks to API)             │
└─────────────────────────────────────────────────────────────┘
```

---

*Generated from learning session on Kubernetes Operator Development*  
*Project: cnat-kubebuilder (Programming Kubernetes)*

