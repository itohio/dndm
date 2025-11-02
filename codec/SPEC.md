# Codec Package Specification

## Overview

The `codec` package provides message encoding and decoding functionality. It handles protobuf message serialization, frame formatting, and buffer management.

## Design Assumptions (Controlled Environment)

This package is designed for controlled environments where:
- Message types are known at compile time (protobuf types)
- Route set is known (no dynamic type discovery needed)
- Message sizes are bounded (known producers)
- No need for complex validation (trusted environment)

## Current Implementation

- Frame format with magic number, sizes, header, and message
- Protobuf-based encoding/decoding
- Buffer pool for efficient memory management
- Support for known types and dynamic type resolution (primarily known types)

## Key Questions

### Encoding Format
1. **Frame Format**:
   - Current: [Magic][TotalSize][HeaderSize][Header][MessageSize][Message]
   - Question: Should we support different frame formats?
   - Question: Should we support frame compression?

2. **Magic Numbers**:
   - Current: 0xFADABEDA, 0xCEBAFE4A
   - Question: Should we support versioning via magic numbers?
   - Question: How to handle magic number collisions?

3. **Size Fields**:
   - Current: 32-bit size fields
   - Question: Should we support variable-length size encoding?
   - Question: How to handle size field overflow?

### Message Encoding
4. **Protobuf Encoding**:
   - Current: Standard protobuf encoding
   - Question: Should we support compression? (gzip, snappy)
   - Question: Should we support different encoding options?

5. **Header Encoding**:
   - Current: Protobuf header encoding
   - Question: Should we support header compression?
   - Question: Should we support optional header fields?

6. **Type Resolution**:
   - Current: Known types map + dynamic resolution
   - Question: Should we support type versioning?
   - Question: How to handle type mismatches?

### Buffer Management
7. **Buffer Pool**:
   - Current: Uses libp2p buffer pool
   - Question: Should we tune pool size?
   - Question: How to handle buffer leaks?

8. **Buffer Lifecycle**:
   - Current: Get/Release pattern
   - Question: Should we support buffer ownership transfer?
   - Question: How to handle buffer cleanup?

9. **Large Messages**:
   - Current: No special handling for large messages
   - Question: Should we support message chunking?
   - Question: How to handle memory limits?

### Performance
10. **Encoding Performance**:
    - Current: Standard protobuf encoding
    - Question: Can we optimize encoding? Use code generation?
    - Question: Should we support zero-copy encoding?

11. **Decoding Performance**:
    - Current: Standard protobuf decoding
    - Question: Can we optimize decoding? Cache reflection?
    - Question: Should we support zero-copy decoding?

12. **Memory Allocations**:
    - Current: Buffer pool reduces allocations
    - Question: Can we reduce allocations further?
    - Question: Should we pre-allocate buffers?

### Error Handling
13. **Malformed Messages**:
    - Current: Returns errors
    - Question: Should we support error recovery?
    - Question: How to handle partial messages?

14. **Size Validation**:
    - Current: Basic size validation
    - Question: Should we support size limits?
    - Question: How to handle oversized messages?

15. **Type Errors**:
    - Current: Returns errors for type mismatches
    - Question: Should we support type conversion?
    - Question: How to handle unknown types?

### Extensibility
16. **Codec Plugins**:
    - Current: Single codec implementation
    - Question: Should we support codec plugins?
    - Question: How to add new codec types?

17. **Versioning**:
    - Current: No versioning support
    - Question: Should we support codec versioning?
    - Question: How to handle version compatibility?

18. **Compression**:
    - Current: No compression
    - Question: Should we support compression?
    - Question: Which compression algorithms?

## Potential Issues

1. **Buffer Leaks**: Buffers may not always be released
2. **Size Limits**: Fixed 32-bit size fields may limit message size
3. **Type Resolution**: Dynamic type resolution may be slow
4. **Error Recovery**: No recovery from malformed messages

## Improvements Needed

1. **Add Compression**: Support for message compression
2. **Improve Buffer Management**: Better buffer lifecycle management
3. **Optimize Encoding/Decoding**: Use code generation or caching
4. **Add Versioning**: Support for codec versioning
5. **Improve Error Handling**: Better error messages and recovery
6. **Add Metrics**: Track encoding/decoding performance

## Protocol Questions

1. **Backward Compatibility**: How to handle protocol changes?
2. **Message Size Limits**: What should be the maximum message size?
3. **Type Evolution**: How to handle protobuf schema evolution?
4. **Compression**: When should compression be used?

