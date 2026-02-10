# runtime.Object Implementation Flow

This document provides a visual representation of how runtime.Object implementation flows through your Kubernetes custom resource.

## The Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        YOUR CODE (types.go)                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  // +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object│
│  type At struct {                                                        │
│      metav1.TypeMeta   `json:",inline"`      ← Provides GetObjectKind() │
│      metav1.ObjectMeta `json:"metadata"`                                │
│      Spec   AtSpec     `json:"spec"`                                    │
│      Status AtStatus   `json:"status"`                                  │
│  }                                                                       │
│                                                                          │
└────────────────────────┬────────────────────────────────────────────────┘
                         │
                         │ Code generation triggered by:
                         │ make generate (or ./hack/update-codegen.sh)
                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│              GENERATED CODE (zz_generated.deepcopy.go)                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  func (in *At) DeepCopyObject() runtime.Object {                        │
│      if c := in.DeepCopy(); c != nil {                                  │
│          return c  ← Returns runtime.Object interface type              │
│      }                                                                   │
│      return nil                                                          │
│  }                                                                       │
│                                                                          │
└────────────────────────┬────────────────────────────────────────────────┘
                         │
                         │ Now At implements runtime.Object interface!
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                  SCHEME REGISTRATION (register.go)                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  func addKnownTypes(scheme *runtime.Scheme) error {                     │
│      scheme.AddKnownTypes(SchemeGroupVersion,                           │
│          &At{},      ← Requires runtime.Object ✓                        │
│          &AtList{},  ← Also runtime.Object ✓                            │
│      )                                                                   │
│      return nil                                                          │
│  }                                                                       │
│                                                                          │
│  Scheme mapping created:                                                │
│  Go Type: *At ←→ GVK: {cnat.../v1alpha1, At}                           │
│                                                                          │
└────────────────────────┬────────────────────────────────────────────────┘
                         │
                         │ Scheme is used by client-gen
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│            GENERATED CLIENT (clientset/.../at.go)                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  type ats struct {                                                       │
│      *gentype.ClientWithList[*At, *AtList]                              │
│  }                                                                       │
│                                                                          │
│  func (c *ats) Create(ctx, at *At, opts) (*At, error) {                │
│      // Under the hood:                                                 │
│      // 1. Calls at.GetObjectKind() to get GVK                          │
│      // 2. Serializes to JSON with apiVersion & kind                    │
│      // 3. POSTs to /apis/cnat.../v1alpha1/namespaces/X/ats            │
│      // 4. Deserializes response using Scheme                           │
│  }                                                                       │
│                                                                          │
└────────────────────────┬────────────────────────────────────────────────┘
                         │
                         │ Client used by controllers
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      CONTROLLER USAGE                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  // Create operation                                                     │
│  at := &At{Spec: AtSpec{...}}                                           │
│  client.CnatV1alpha1().Ats("default").Create(ctx, at, opts)             │
│                           Uses ↑                                         │
│                      GetObjectKind()                                     │
│                                                                          │
│  // Informer caching                                                     │
│  informer.AddEventHandler(cache.ResourceEventHandlerFuncs{              │
│      UpdateFunc: func(old, new interface{}) {                           │
│          oldAt := old.(runtime.Object).DeepCopyObject()                 │
│                                 Uses ↑                                   │
│                          DeepCopyObject()                                │
│      },                                                                  │
│  })                                                                      │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## The Two Methods of runtime.Object

```
┌───────────────────────────────────────────────────────────────┐
│         runtime.Object Interface Requirements                 │
├───────────────────────────────────────────────────────────────┤
│                                                                │
│  Method 1: GetObjectKind() schema.ObjectKind                  │
│  ├─ Provided by: Embedding metav1.TypeMeta                    │
│  ├─ Purpose: Type identity (Group, Version, Kind)             │
│  └─ Used in: Serialization/Deserialization                    │
│                                                                │
│  Method 2: DeepCopyObject() runtime.Object                    │
│  ├─ Provided by: Auto-generated code                          │
│  ├─ Purpose: Safe cloning without reflection                  │
│  └─ Used in: Caches, Controllers, Informers                   │
│                                                                │
└───────────────────────────────────────────────────────────────┘
```

## Serialization Flow

```
USER CREATE REQUEST
        │
        ▼
┌─────────────────┐
│  At struct      │  Your Go object in memory
│  {              │
│    TypeMeta: {} │  ← GetObjectKind() reads this
│    ObjectMeta   │
│    Spec: {...}  │
│  }              │
└────────┬────────┘
         │
         │ Client.Create() called
         │
         ▼
┌─────────────────┐
│ GetObjectKind() │  Returns: Group=cnat..., Version=v1alpha1, Kind=At
└────────┬────────┘
         │
         │ Serializer uses this to construct JSON
         │
         ▼
┌─────────────────────────┐
│  JSON sent to server    │
│  {                      │
│    "apiVersion": "cnat.programming-kubernetes.info/v1alpha1"
│    "kind": "At"         │
│    "metadata": {...}    │
│    "spec": {...}        │
│  }                      │
└─────────┬───────────────┘
          │
          │ HTTP POST /apis/cnat.../v1alpha1/namespaces/default/ats
          │
          ▼
    KUBERNETES API SERVER
```

## Deserialization Flow

```
KUBERNETES API SERVER
        │
        │ HTTP Response with JSON
        │
        ▼
┌─────────────────────────┐
│  JSON from server       │
│  {                      │
│    "apiVersion": "cnat.programming-kubernetes.info/v1alpha1"
│    "kind": "At"         │
│    "metadata": {...}    │
│    "spec": {...}        │
│    "status": {...}      │
│  }                      │
└────────┬────────────────┘
         │
         │ Client reads apiVersion and kind
         │
         ▼
┌─────────────────────┐
│ Scheme Lookup       │  Scheme.New(GVK{cnat.../v1alpha1, At})
│ "cnat.../v1alpha1"  │  → Returns: &At{} (empty instance)
│ + "At"              │
│ → *At type found    │
└────────┬────────────┘
         │
         │ Deserialize JSON into *At
         │
         ▼
┌─────────────────┐
│  At struct      │  Your Go object reconstructed
│  {              │
│    TypeMeta     │
│    ObjectMeta   │
│    Spec: {...}  │
│    Status: {...}│
│  }              │
└─────────────────┘
         │
         │ Returned to caller
         │
         ▼
    YOUR CONTROLLER CODE
```

## Deep Copy Flow (Informer Cache)

```
KUBERNETES WATCH EVENT
        │
        ▼
┌────────────────────┐
│ New At object      │  Received from watch stream
│ (from API server)  │
└────────┬───────────┘
         │
         │ Informer needs to cache this
         │
         ▼
┌────────────────────┐
│ cache.Store        │  Generic cache stores runtime.Object
│ Add(obj)           │
└────────┬───────────┘
         │
         │ Store casts to runtime.Object
         │
         ▼
┌────────────────────────┐
│ obj.DeepCopyObject()   │  Creates independent copy
└────────┬───────────────┘
         │
         │ Calls the generated method
         │
         ▼
┌────────────────────────┐
│ func (in *At)          │
│   DeepCopyObject()     │
│   runtime.Object {     │
│     return in.DeepCopy()  ← Type-safe deep copy
│   }                    │
└────────┬───────────────┘
         │
         │ Returns new *At
         │
         ▼
┌────────────────────┐
│ Cached At object   │  Safe to modify without affecting original
└────────────────────┘
         │
         │ Controller gets object from cache
         │
         ▼
    CONTROLLER LOGIC
    (Can modify safely)
```

## Why runtime.Object is Required at Each Layer

```
┌──────────────────────────────────────────────────────────────┐
│ Layer                │ Requires runtime.Object Because...     │
├──────────────────────────────────────────────────────────────┤
│ Scheme Registration  │ AddKnownTypes() signature requires it  │
│                      │ → Compile-time type safety             │
├──────────────────────────────────────────────────────────────┤
│ Serialization        │ Need GetObjectKind() to set apiVersion │
│                      │ and kind in JSON/YAML                  │
├──────────────────────────────────────────────────────────────┤
│ Deserialization      │ Scheme uses GVK to lookup Go type      │
│                      │ → Creates correct struct instance      │
├──────────────────────────────────────────────────────────────┤
│ Client Generation    │ gentype.Client requires both *T and    │
│                      │ *TList to be runtime.Object            │
├──────────────────────────────────────────────────────────────┤
│ Informer/Cache       │ Needs DeepCopyObject() to clone        │
│                      │ objects safely for concurrent access   │
├──────────────────────────────────────────────────────────────┤
│ Controller-Runtime   │ All reconcilers work with              │
│                      │ runtime.Object for type generality     │
└──────────────────────────────────────────────────────────────┘
```

## The Magic Combo: TypeMeta + Code Generation

```
┌─────────────────────────────────────────────────────────────────┐
│                    metav1.TypeMeta                               │
│  ┌───────────────────────────────────────────────────┐          │
│  │ type TypeMeta struct {                            │          │
│  │     APIVersion string                              │          │
│  │     Kind       string                              │          │
│  │ }                                                  │          │
│  │                                                    │          │
│  │ func (obj *TypeMeta) GetObjectKind()              │          │
│  │     schema.ObjectKind {                           │          │
│  │     return obj  // TypeMeta implements ObjectKind │          │
│  │ }                                                  │          │
│  └───────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
                             +
┌─────────────────────────────────────────────────────────────────┐
│        +k8s:deepcopy-gen:interfaces=runtime.Object               │
│  ┌───────────────────────────────────────────────────┐          │
│  │ // Generated in zz_generated.deepcopy.go           │          │
│  │ func (in *At) DeepCopyObject() runtime.Object {   │          │
│  │     if c := in.DeepCopy(); c != nil {             │          │
│  │         return c                                   │          │
│  │     }                                              │          │
│  │     return nil                                     │          │
│  │ }                                                  │          │
│  └───────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
                             =
┌─────────────────────────────────────────────────────────────────┐
│                   FULLY IMPLEMENTS runtime.Object                │
│                                                                  │
│  ✓ GetObjectKind() schema.ObjectKind                            │
│  ✓ DeepCopyObject() runtime.Object                              │
│                                                                  │
│  → Can be registered in Scheme                                  │
│  → Can be serialized/deserialized                               │
│  → Can be cloned safely                                         │
│  → Can be used in clients, informers, controllers               │
└─────────────────────────────────────────────────────────────────┘
```

## Error Without runtime.Object

```
❌ NO TypeMeta EMBEDDED
┌──────────────────────┐
│ type At struct {     │
│   ObjectMeta         │  Missing TypeMeta!
│   Spec AtSpec        │
│ }                    │
└──────────────────────┘
         │
         ▼
Compile Error: *At does not implement runtime.Object
               (missing GetObjectKind method)

❌ NO DEEPCOPY-GEN TAG
┌──────────────────────┐
│ type At struct {     │
│   TypeMeta           │  Has TypeMeta but no tag!
│   ObjectMeta         │
│   Spec AtSpec        │
│ }                    │
└──────────────────────┘
         │
         ▼
Compile Error: *At does not implement runtime.Object
               (missing DeepCopyObject method)

❌ FORGOT TO RUN CODE GENERATION
┌──────────────────────┐
│ // +k8s:deepcopy-gen │
│ type At struct {     │
│   TypeMeta           │  Tag exists but code not generated!
│   ObjectMeta         │
│   Spec AtSpec        │
│ }                    │
└──────────────────────┘
         │
         ▼
Compile Error: *At does not implement runtime.Object
               (missing DeepCopyObject method)
Solution: Run `make generate` or `./hack/update-codegen.sh`
```

## Quick Reference

### To make a type implement runtime.Object:

1. ✅ Embed `metav1.TypeMeta`
2. ✅ Add `// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object`
3. ✅ Run code generation: `make generate`
4. ✅ Register in Scheme: `scheme.AddKnownTypes(..., &YourType{})`

### To verify it works:

```go
var _ runtime.Object = &At{}      // Compile-time check
var _ runtime.Object = &AtList{}  // Compile-time check
```

If these lines compile, your types correctly implement runtime.Object!

