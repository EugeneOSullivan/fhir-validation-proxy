# FHIR Validation Proxy - Requirements & Architecture Plan

## Executive Summary

The FHIR Validation Proxy is a **high-throughput, low-latency validation layer** that sits between API Management and Google Cloud Healthcare FHIR Store. It validates FHIR resources in-memory before forwarding valid requests to the store.

**Key Requirements:**
- **Latency**: Sub-100ms validation for simple resources, <500ms for complex bundles
- **Throughput**: 1000+ requests/second per instance
- **In-Memory Validation**: All profiles, rules, and recipes loaded at startup
- **Reference Data Caching**: Practitioner, PractitionerRole, Organization, Location
- **Bundle Type Validation**: Support multiple message bundle definitions
- **Platform Security**: Service-to-service auth (API Management handles user auth)

---

## Architecture Decision: Cloud Run vs Dataflow

### ✅ **Cloud Run is the Correct Choice**

**Why Cloud Run:**
- **Request/Response Pattern**: This is a synchronous HTTP proxy, not batch processing
- **Low Latency**: Cloud Run provides sub-100ms cold starts with Gen2
- **Auto-scaling**: Handles traffic spikes automatically (1-100 instances)
- **Cost-effective**: Pay per request, no idle costs
- **Stateless**: Perfect for validation proxy pattern

**Why NOT Dataflow:**
- **Batch Processing**: Dataflow is for batch/streaming data pipelines
- **Higher Latency**: Not designed for request/response patterns
- **Overhead**: More complex setup for simple validation needs
- **Cost**: More expensive for request/response workloads

**Recommendation**: Continue with Cloud Run, optimize for:
- Gen2 execution environment (faster cold starts)
- Startup CPU boost (faster initialization)
- Minimum instances = 1 (eliminate cold starts)
- High concurrency (80-100 requests per instance)

---

## Key Requirements

### 1. **Latency Optimization** (CRITICAL)

**Targets:**
- Simple resource validation: < 50ms
- Complex bundle validation: < 300ms
- Total proxy overhead: < 100ms

**Strategies:**
- ✅ In-memory profiles (already implemented)
- ✅ In-memory rules (already implemented)
- ✅ In-memory recipes (already implemented)
- 🔄 **Reference data caching** (to be implemented)
- 🔄 **Connection pooling** (to be implemented)
- 🔄 **Async validation** for large bundles (optional)

### 2. **Reference Data Caching** (HIGH PRIORITY)

**Cached Resources:**
- `Practitioner` - Healthcare providers
- `PractitionerRole` - Provider roles
- `Organization` - Healthcare organizations
- `Location` - Physical locations

**Cache Strategy:**
- **TTL**: 1 hour (configurable)
- **Refresh**: Background refresh before expiration
- **Storage**: In-memory with sync.RWMutex for thread-safety
- **Key**: Resource type + ID (e.g., `Practitioner/123`)
- **Fallback**: Fetch from FHIR store if cache miss

**Implementation:**
```go
type ReferenceDataCache struct {
    cache map[string]CachedResource
    mu    sync.RWMutex
    ttl   time.Duration
}

type CachedResource struct {
    Resource  map[string]interface{}
    ExpiresAt time.Time
}
```

### 3. **Bundle Type Validation** (HIGH PRIORITY)

**Requirements:**
- Support multiple message bundle types
- Validate based on bundle type (transaction, message, batch, etc.)
- Different validation rules per bundle type
- Recipe-based validation per bundle type

**Current State:**
- ✅ Supports transaction bundles
- ✅ Supports message bundles
- 🔄 **Needs**: Bundle type detection and routing

**Implementation:**
```go
func ValidateBundle(bundle map[string]interface{}) ValidationResult {
    bundleType := bundle["type"].(string)
    
    switch bundleType {
    case "transaction":
        return ValidateTransactionBundle(bundle)
    case "message":
        return ValidateMessageBundle(bundle)
    case "batch":
        return ValidateBatchBundle(bundle)
    default:
        return ValidateGenericBundle(bundle)
    }
}
```

### 4. **Platform Security** (MEDIUM PRIORITY)

**Since API Management handles user auth:**
- ✅ Service-to-service authentication (service account)
- ✅ Request validation (size, content)
- ✅ Input sanitization
- 🔄 **Rate limiting** (optional, API Management may handle)
- 🔄 **Request signing** (optional, for service-to-service)

**Security Requirements:**
- Validate request size (10MB limit)
- Validate resource types (prevent injection)
- Sanitize IDs (prevent path traversal)
- Audit logging (structured logs)

### 5. **In-Memory Validation** (ALREADY IMPLEMENTED)

**Current State:**
- ✅ Profiles loaded at startup (`LoadProfiles`)
- ✅ Rules loaded at startup (`LoadRules`)
- ✅ Recipes loaded at startup (`LoadRecipes`)

**Optimizations:**
- ✅ Caching (already implemented)
- 🔄 **Pre-compile regex patterns** (performance)
- 🔄 **Index profiles by resource type** (faster lookup)

---

## Performance Optimizations

### 1. **Reference Data Caching**

**Implementation Priority: HIGH**

**Benefits:**
- Eliminates FHIR store lookups for reference validation
- Reduces latency by 50-200ms per reference check
- Reduces load on FHIR store

**Cache Strategy:**
- **Write-through**: Update cache on write operations
- **Refresh-ahead**: Background refresh before expiration
- **LRU eviction**: If memory limits reached

### 2. **Connection Pooling**

**Implementation Priority: HIGH**

**Current State:**
- Basic HTTP client with 30s timeout
- No connection pooling configured

**Optimization:**
```go
httpClient: &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        DisableKeepAlives:   false,
    },
}
```

**Benefits:**
- Reuse connections (eliminates TCP handshake)
- Reduces latency by 10-50ms per request
- Reduces server load

### 3. **Profile Indexing**

**Implementation Priority: MEDIUM**

**Current State:**
- Profiles indexed by URL
- Lookup requires iteration

**Optimization:**
```go
type ProfileIndex struct {
    ByResourceType map[string][]StructureDefinition
    ByURL         map[string]StructureDefinition
}
```

**Benefits:**
- O(1) lookup by resource type
- Faster profile matching
- Reduces validation time by 5-10ms

### 4. **Regex Pre-compilation**

**Implementation Priority: MEDIUM**

**Current State:**
- Regex compiled on each validation

**Optimization:**
```go
var compiledPatterns = map[string]*regexp.Regexp{}

func getCompiledPattern(pattern string) *regexp.Regexp {
    if re, ok := compiledPatterns[pattern]; ok {
        return re
    }
    re := regexp.MustCompile(pattern)
    compiledPatterns[pattern] = re
    return re
}
```

**Benefits:**
- Eliminates regex compilation overhead
- Reduces validation time by 2-5ms per pattern

### 5. **Async Validation** (Optional)

**Implementation Priority: LOW**

**Use Case:**
- Large bundles (>100 entries)
- Complex validation rules

**Strategy:**
- Validate in background
- Return immediately if simple checks pass
- Stream validation results

**Trade-off:**
- Adds complexity
- May not be needed if validation is fast enough

---

## Caching Strategy

### 1. **Reference Data Cache**

**Resources Cached:**
- `Practitioner/{id}`
- `PractitionerRole/{id}`
- `Organization/{id}`
- `Location/{id}`

**TTL:** 1 hour (configurable)

**Refresh Strategy:**
- Background refresh at 90% TTL
- Write-through on updates
- Invalidate on delete

**Memory Management:**
- LRU eviction if memory limits reached
- Configurable max cache size
- Monitor cache hit rate

### 2. **Profile Cache** (Already Implemented)

**Current State:**
- Profiles loaded at startup
- Cached in memory

**Optimization:**
- Index by resource type
- Pre-compile validation rules

### 3. **Rules Cache** (Already Implemented)

**Current State:**
- Rules loaded at startup
- Cached in memory

**Optimization:**
- Pre-compile regex patterns
- Index by resource type

### 4. **Capability Statement Cache**

**Implementation Priority: LOW**

**Use Case:**
- `/metadata` endpoint called frequently
- Response is static

**Strategy:**
- Cache capability statement
- Refresh on startup
- TTL: 24 hours

---

## Implementation Plan

### Phase 1: Critical Optimizations (Week 1)

1. **Reference Data Caching**
   - Implement cache structure
   - Add cache lookup in validation
   - Add cache refresh logic
   - Add cache metrics

2. **Connection Pooling**
   - Configure HTTP transport
   - Add connection metrics
   - Monitor connection usage

3. **Bundle Type Validation**
   - Add bundle type detection
   - Route to appropriate validator
   - Support multiple message types

### Phase 2: Performance Improvements (Week 2)

4. **Profile Indexing**
   - Index profiles by resource type
   - Optimize profile lookup
   - Add profile metrics

5. **Regex Pre-compilation**
   - Pre-compile all patterns
   - Cache compiled patterns
   - Add pattern metrics

6. **Request Body Optimization**
   - Read once, reuse
   - Stream large bodies
   - Add body size metrics

### Phase 3: Monitoring & Observability (Week 3)

7. **Structured Logging**
   - JSON logs
   - Request tracing
   - Error correlation

8. **Metrics Enhancement**
   - Cache hit rates
   - Validation duration by type
   - Connection pool usage
   - Reference data cache stats

9. **Health Checks**
   - Cache health
   - Connection pool health
   - Upstream health

---

## Success Metrics

### Latency Targets
- **Simple resource**: < 50ms (p95)
- **Complex bundle**: < 300ms (p95)
- **Total proxy overhead**: < 100ms (p95)

### Throughput Targets
- **Requests/second**: 1000+ per instance
- **Concurrent requests**: 80-100 per instance
- **Cache hit rate**: > 80% for reference data

### Reliability Targets
- **Availability**: 99.9%
- **Error rate**: < 0.1%
- **Validation accuracy**: 100%

---

## Configuration

### Cache Configuration

```yaml
cache:
  reference_data:
    enabled: true
    ttl: 3600s  # 1 hour
    max_size: 10000  # Max cached resources
    refresh_threshold: 0.9  # Refresh at 90% TTL
    resources:
      - Practitioner
      - PractitionerRole
      - Organization
      - Location
```

### Performance Configuration

```yaml
performance:
  connection_pool:
    max_idle_conns: 100
    max_idle_conns_per_host: 10
    idle_conn_timeout: 90s
  validation:
    async_threshold: 100  # Use async for bundles > 100 entries
    timeout: 30s
```

---

## Next Steps

1. **Review** this requirements document
2. **Prioritize** implementation phases
3. **Implement** Phase 1 optimizations
4. **Test** performance improvements
5. **Monitor** production metrics
6. **Iterate** based on results

---

*Last Updated: 2024*
*Version: 1.0*


