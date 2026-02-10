# Understanding runtime.Object in Kubernetes

This document explains how the `runtime.Object` interface enables your Go structs to work as Kubernetes resources.

## The Big Picture

When you define a custom Kubernetes resource (like our `At` type), you need to satisfy several requirements for it to work with the Kubernetes API machinery. The `runtime.Object` interface is the mechanism that makes this work.

## What is runtime.Object?

The `runtime.Object` interface is defined in `k8s.io/apimachinery/pkg/runtime`:

```go
type Object interface {
    GetObjectKind() schema.ObjectKind
    DeepCopyObject() Object
}
```

It requires two methods:
1. **GetObjectKind()** - Returns type identity (Group, Version, Kind)
2. **DeepCopyObject()** - Creates a deep copy of the object

## Why Does This Matter?

### 1. Registration with the Scheme

**The Problem:**
- The Kubernetes Scheme maps Go types to GroupVersionKinds (GVKs)
- Example: `v1.Pod` → `{Group: "", Version: "v1", Kind: "Pod"}`

**The Constraint:**
- `scheme.AddKnownTypes()` ONLY accepts types that implement `runtime.Object`
- See `register.go:addKnownTypes()` where we call:
  ```go
  scheme.AddKnownTypes(SchemeGroupVersion, &At{}, &AtList{})
  ```

**The Result:**
- Without `runtime.Object`, your type cannot be registered
- Without registration, the client libraries cannot encode/decode your type

### 2. Handling Versioning and Typing on the Wire

**The Problem:**
- When sent over the network, Kubernetes objects need to carry their identity
- JSON/YAML must include `apiVersion` and `kind` fields

**The Solution:**
- The `GetObjectKind()` method (from `runtime.Object`) enables this
- We satisfy this by embedding `metav1.TypeMeta` in our structs:
  ```go
  type At struct {
      metav1.TypeMeta   `json:",inline"`  // Provides GetObjectKind()
      metav1.ObjectMeta `json:"metadata,omitempty"`
      Spec   AtSpec     `json:"spec,omitempty"`
      Status AtStatus   `json:"status,omitempty"`
  }
  ```

**What Happens:**
- During serialization: TypeMeta sets `apiVersion` and `kind` in the JSON
- During deserialization: Scheme uses these fields to determine which Go type to use

### 3. Generic Deep Copying (Performance & Safety)

**The History:**
- Originally, Kubernetes used Go reflection to copy objects
- This was slow and caused hard-to-debug bugs

**The Problem:**
- Controllers need to copy objects frequently:
  - Before modifying them (to avoid cache corruption)
  - When processing watch events
  - When passing objects between goroutines

**The Solution:**
- Kubernetes switched to static, compiled deep copying
- The `DeepCopyObject()` method (from `runtime.Object`) enables this
- We generate this method using code generation tags

## How We Implement runtime.Object

### Step 1: Embed metav1.TypeMeta (in types.go)

```go
type At struct {
    metav1.TypeMeta   `json:",inline"`  // ← Provides GetObjectKind()
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   AtSpec     `json:"spec,omitempty"`
    Status AtStatus   `json:"status,omitempty"`
}
```

**What this does:**
- `metav1.TypeMeta` has `GetObjectKind()` method
- By embedding it, `At` now has this method too
- ✅ First requirement of `runtime.Object` satisfied!

### Step 2: Add Code Generation Tag (in types.go)

```go
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type At struct {
    // ... fields ...
}
```

**What this does:**
- Tells `deepcopy-gen` to generate `DeepCopyObject() runtime.Object` method
- The generated code is in `zz_generated.deepcopy.go`
- ✅ Second requirement of `runtime.Object` satisfied!

### Step 3: Register with the Scheme (in register.go)

```go
func addKnownTypes(scheme *runtime.Scheme) error {
    scheme.AddKnownTypes(SchemeGroupVersion,
        &At{},     // Now implements runtime.Object ✅
        &AtList{}, // Also implements runtime.Object ✅
    )
    return nil
}
```

**What this does:**
- Maps the Go type `At` to GVK `{cnat.programming-kubernetes.info, v1alpha1, At}`
- Now the Scheme knows how to encode/decode `At` objects

## The Generated Code (zz_generated.deepcopy.go)

For each type marked with `+k8s:deepcopy-gen:interfaces=runtime.Object`, three methods are generated:

### 1. DeepCopyInto(*T)
```go
func (in *At) DeepCopyInto(out *At) {
    *out = *in
    out.TypeMeta = in.TypeMeta
    in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)  // Handles pointers/maps
    out.Spec = in.Spec
    out.Status = in.Status
}
```
- Low-level method that does the actual copying
- Handles complex fields (pointers, slices, maps) properly

### 2. DeepCopy() *T
```go
func (in *At) DeepCopy() *At {
    if in == nil {
        return nil
    }
    out := new(At)
    in.DeepCopyInto(out)
    return out
}
```
- Convenience method that allocates a new instance and calls DeepCopyInto
- Type-specific return type (*At)

### 3. DeepCopyObject() runtime.Object ⭐
```go
func (in *At) DeepCopyObject() runtime.Object {
    if c := in.DeepCopy(); c != nil {
        return c
    }
    return nil
}
```
- **THIS IS THE KEY METHOD** for runtime.Object!
- Returns `runtime.Object` (interface type) instead of `*At`
- Allows generic code to copy ANY Kubernetes object:
  ```go
  var obj runtime.Object = &At{...}
  clone := obj.DeepCopyObject()  // Works without knowing specific type!
  ```

## How This Enables the Generated Client

See `pkg/generated/clientset/versioned/typed/cnat/v1alpha1/at.go`:

```go
gentype.NewClientWithList[*At, *AtList](
    "ats",
    c.RESTClient(),
    scheme.ParameterCodec,  // ← Uses the Scheme we registered At in
    namespace,
    func() *At { return &At{} },
    func() *AtList { return &AtList{} },
)
```

**How runtime.Object is used here:**

1. **Create/Update operations:**
   - Client calls `GetObjectKind()` to get the GVK
   - Serializes the object to JSON with correct `apiVersion` and `kind`

2. **Get/List operations:**
   - Server returns JSON with `apiVersion` and `kind`
   - Client looks up the GVK in the Scheme
   - Deserializes into the correct Go type (*At or *AtList)

3. **Watch operations:**
   - Events stream in from the server
   - Each event object is cloned using `DeepCopyObject()`
   - Informer caches use these clones to avoid race conditions

## Summary: The Three Pillars

You implement `runtime.Object` so that your struct can be:

1. **Mapped to a Kubernetes resource (GVK)** by the Scheme
   - Via: Embedding `metav1.TypeMeta` (provides `GetObjectKind()`)
   - Registered in: `register.go:addKnownTypes()`

2. **Serialized correctly** with kind and apiVersion fields
   - Via: Embedding `metav1.TypeMeta`
   - Used in: Client Create/Update/Get operations

3. **Cloned efficiently and safely** by generic controllers and caches
   - Via: Auto-generated `DeepCopyObject()` method
   - Generated by: `+k8s:deepcopy-gen:interfaces=runtime.Object` tag
   - Used in: Informers, controllers, anywhere objects need to be copied

## What You DON'T Write Manually

✅ You write:
- The struct definition (`type At struct { ... }`)
- The code generation tags (`// +k8s:deepcopy-gen:interfaces=...`)
- The Scheme registration (`scheme.AddKnownTypes(...)`)

❌ You DON'T write:
- `GetObjectKind()` - provided by embedded `metav1.TypeMeta`
- `DeepCopyObject()` - auto-generated by `deepcopy-gen`
- `DeepCopy()` - auto-generated by `deepcopy-gen`
- `DeepCopyInto()` - auto-generated by `deepcopy-gen`

## Common Mistakes

### Mistake 1: Forgetting to embed metav1.TypeMeta
```go
// ❌ WRONG: No TypeMeta
type At struct {
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec AtSpec `json:"spec,omitempty"`
}
// Error: At does not implement GetObjectKind()
```

### Mistake 2: Forgetting the deepcopy-gen tag
```go
// ❌ WRONG: No code generation tag
type At struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec AtSpec `json:"spec,omitempty"`
}
// Error: At does not implement DeepCopyObject()
```

### Mistake 3: Not running code generation
```bash
# ❌ WRONG: Edited types.go but didn't regenerate
vim pkg/apis/cnat/v1alpha1/types.go
# zz_generated.deepcopy.go is now out of date!

# ✅ CORRECT: Always regenerate after changing types
vim pkg/apis/cnat/v1alpha1/types.go
make generate  # or ./hack/update-codegen.sh
```

## Verification Checklist

To verify your type properly implements runtime.Object:

- [ ] `types.go`: Embeds `metav1.TypeMeta`
- [ ] `types.go`: Has `+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object` tag
- [ ] `zz_generated.deepcopy.go`: Contains `DeepCopyObject() runtime.Object` method
- [ ] `register.go`: Calls `scheme.AddKnownTypes()` with your type
- [ ] Compiles without errors: `go build ./pkg/apis/cnat/v1alpha1`

## Further Reading

- **Chapter 3** of "Programming Kubernetes": Scheme and runtime.Object details
- **Chapter 5** of "Programming Kubernetes": Client generation and usage
- Kubernetes API conventions: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md

