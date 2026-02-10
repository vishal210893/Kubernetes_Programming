# Understanding the Reconcile Function

## What is Reconciliation?

**Reconciliation** is the heart of the Kubernetes controller pattern. It's a control loop that continuously works to make the **actual state** match the **desired state**.

```
┌─────────────────────────────────────────────────────┐
│          THE RECONCILIATION LOOP                    │
│                                                     │
│  ┌──────────┐      ┌────────────┐      ┌────────┐ │
│  │ Desired  │ ───▶ │ Reconcile  │ ───▶ │ Actual │ │
│  │  State   │      │  Function  │      │ State  │ │
│  │  (Spec)  │      │            │      │(Status)│ │
│  └──────────┘      └────────────┘      └────────┘ │
│       ▲                  │                   │     │
│       │                  │                   │     │
│       └──────────────────┴───────────────────┘     │
│              Keep comparing and adjusting          │
└─────────────────────────────────────────────────────┘
```

---

## When Does Reconcile Run?

The `Reconcile()` function is called automatically by the controller-runtime framework when:

### 1. **Resource Events** (Create/Update/Delete)
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

### 2. **Owned Resource Changes**
When a Pod (owned by our At resource) changes:
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

### 3. **RequeueAfter Timer**
When we return `Result{RequeueAfter: 5*time.Minute}`:
→ Kubernetes waits 5 minutes, then calls Reconcile again

### 4. **Error Backoff**
When we return an error:
→ Kubernetes retries with exponential backoff (1s, 2s, 4s, 8s, ...)

---

## reconcile.Result - The Return Value

The `Result` struct tells Kubernetes **when to call Reconcile again**:

```go
type Result struct {
    Requeue      bool          // Should we requeue immediately?
    RequeueAfter time.Duration // Requeue after this duration
}
```

### Return Value Behaviors

#### 1. `return reconcile.Result{}, nil`
**Meaning**: "Everything is good, don't requeue"
```go
// Example: Resource is in DONE state
case cnatv1alpha1.PhaseDone:
    return reconcile.Result{}, nil
```
- ✅ No error
- ⏸️ **Won't requeue automatically**
- 🔔 Will only run again if:
  - Someone edits the At resource
  - A Pod owned by At changes

---

#### 2. `return reconcile.Result{}, err`
**Meaning**: "Something went wrong, retry with backoff"
```go
// Example: Failed to parse schedule
d, err := timeUntilSchedule(instance.Spec.Schedule)
if err != nil {
    return reconcile.Result{}, err
}
```
- ❌ Error occurred
- 🔄 **Requeues with exponential backoff**:
  - 1st retry: ~1 second
  - 2nd retry: ~2 seconds
  - 3rd retry: ~4 seconds
  - ...up to 5 minutes max
- 📝 Error is logged

---

#### 3. `return reconcile.Result{RequeueAfter: duration}, nil`
**Meaning**: "Everything is good, but check again after X time"
```go
// Example: Schedule is in 5 minutes
if d > 0 {
    return reconcile.Result{RequeueAfter: d}, nil
}
```
- ✅ No error
- ⏰ **Requeues after exact duration**
- 🎯 Perfect for time-based operations
- 💡 More efficient than polling!

---

#### 4. `return reconcile.Result{Requeue: true}, nil`
**Meaning**: "Everything is good, but requeue immediately"
```go
// Rarely used - immediate retry without error
return reconcile.Result{Requeue: true}, nil
```
- ✅ No error
- 🔄 **Requeues immediately**
- ⚠️ Use sparingly (can cause tight loops)

---

#### 5. `return reconcile.Result{RequeueAfter: duration}, err`
**Meaning**: "Error wins, RequeueAfter is ignored"
```go
// Error takes precedence
return reconcile.Result{RequeueAfter: 1*time.Hour}, fmt.Errorf("oops")
```
- ❌ Error occurred
- 🔄 **Uses error backoff** (ignores RequeueAfter)

---

## State Machine Flow in Our Controller

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

---

## Detailed Example: Scheduling a Command

Let's trace what happens when you create an At resource:

### Step 1: Create Resource
```bash
kubectl apply -f - <<EOF
apiVersion: cnat.programming-kubernetes.info/v1alpha1
kind: At
metadata:
  name: demo
spec:
  schedule: "2026-02-10T20:35:00Z"  # 5 minutes from now
  command: "echo Hello Kubernetes"
EOF
```

### Step 2: First Reconcile (t=0)
```go
// Reconcile called immediately after creation
instance.Status.Phase = ""  // Empty initially
→ Set to PENDING

// Calculate time until schedule
d = 5 minutes

// Return: Schedule reconcile for 5 minutes later
return reconcile.Result{RequeueAfter: 5*time.Minute}, nil
```
**Result**: Controller sleeps for 5 minutes ⏰

---

### Step 3: Second Reconcile (t=5min)
```go
// Reconcile called after RequeueAfter expires
instance.Status.Phase = PENDING

// Calculate time until schedule
d = 0 seconds (time has arrived!)

// Transition to RUNNING
instance.Status.Phase = RUNNING

// Update status in Kubernetes
r.Status().Update(context.TODO(), instance)

return reconcile.Result{}, nil
```
**Result**: Status updated, controller waits for changes 🔔

---

### Step 4: Third Reconcile (t=5min + 10ms)
```go
// Reconcile called because status changed (PENDING → RUNNING)
instance.Status.Phase = RUNNING

// Create Pod
pod := newPodForCR(instance)
r.Create(context.TODO(), pod)

// Pod created successfully
return reconcile.Result{}, nil
```
**Result**: Pod created, controller waits for Pod events 🔔

---

### Step 5: Fourth Reconcile (t=5min + 2sec)
```go
// Reconcile called because Pod status changed (Created → Running)
instance.Status.Phase = RUNNING

// Check Pod status
found.Status.Phase = corev1.PodRunning

// Pod still running, wait
return reconcile.Result{}, nil
```
**Result**: Controller waits for Pod to finish 🔔

---

### Step 6: Fifth Reconcile (t=5min + 5sec)
```go
// Reconcile called because Pod status changed (Running → Succeeded)
instance.Status.Phase = RUNNING

// Check Pod status
found.Status.Phase = corev1.PodSucceeded

// Transition to DONE
instance.Status.Phase = DONE

// Update status
r.Status().Update(context.TODO(), instance)

return reconcile.Result{}, nil
```
**Result**: Command executed successfully! 🎉

---

### Step 7: Sixth Reconcile (t=5min + 5sec + 10ms)
```go
// Reconcile called because status changed (RUNNING → DONE)
instance.Status.Phase = DONE

// Nothing to do
return reconcile.Result{}, nil
```
**Result**: Reconciliation complete ✅

---

## Key Insights

### 1. **Reconcile is Event-Driven**
- Not a polling loop
- Triggered by events (resource changes, owned resources, timers)
- Very efficient!

### 2. **Reconcile Must Be Idempotent**
- Can be called multiple times for the same state
- Must handle "already exists" scenarios gracefully
- Example: If Pod exists, don't try to create it again

### 3. **Result Controls Timing**
```go
// DON'T do this (blocking):
time.Sleep(5 * time.Minute)  // ❌ Blocks the controller!

// DO this instead:
return reconcile.Result{RequeueAfter: 5*time.Minute}, nil  // ✅ Efficient!
```

### 4. **Errors Trigger Automatic Retry**
- Don't need to manually implement retry logic
- Exponential backoff prevents API server overload
- Transient errors often resolve themselves

### 5. **Owner References Enable Cascading**
```go
controllerutil.SetControllerReference(instance, pod, r.Scheme)
```
- When At is deleted → Pod is automatically deleted (Garbage Collection)
- Pod changes trigger At reconciliation

---

## Common Patterns

### Pattern 1: Time-Based Operations
```go
// Schedule something for the future
timeUntil := targetTime.Sub(time.Now())
if timeUntil > 0 {
    return reconcile.Result{RequeueAfter: timeUntil}, nil
}
```

### Pattern 2: Wait for External State
```go
// Don't poll - let events trigger reconciliation
if pod.Status.Phase == corev1.PodRunning {
    // Still running, wait for next event
    return reconcile.Result{}, nil
}
```

### Pattern 3: Retry on Error
```go
// Let the framework handle retries
obj, err := externalAPI.Get()
if err != nil {
    return reconcile.Result{}, err  // Auto retry with backoff
}
```

### Pattern 4: Periodic Sync
```go
// Recheck every 30 seconds (e.g., for polling external state)
return reconcile.Result{RequeueAfter: 30*time.Second}, nil
```

---

## Debugging Tips

### 1. Add Logging
```go
reqLogger.Info("=== Reconciling At", "phase", instance.Status.Phase)
```

### 2. Watch Controller Logs
```bash
kubectl logs -f deployment/controller-manager -n system
```

### 3. Check Resource Status
```bash
kubectl get at demo -o yaml
kubectl describe at demo
```

### 4. Watch Events
```bash
kubectl get events --watch
```

---

## Summary

| Return Value | Behavior | Use Case |
|-------------|----------|----------|
| `Result{}, nil` | Success, don't requeue | Resource is stable |
| `Result{}, err` | Error, retry with backoff | Transient failures |
| `Result{RequeueAfter: d}, nil` | Success, requeue after duration | Time-based operations |
| `Result{Requeue: true}, nil` | Success, requeue immediately | Force retry (rare) |

**The reconciliation loop is the foundation of Kubernetes controllers - master it, and you master Kubernetes programming!** 🚀

