/// Metadata attached to AG-UI events, messages, and tool calls.
library;

/// Open metadata keyed by strings with JSON-compatible values.
typedef Metadata = Map<String, dynamic>;

/// The key reserved for AG-UI's own metadata conventions.
const String agUiMetadataKey = 'ag-ui';

/// Shallowly merges [incoming] into [existing], with incoming values winning.
///
/// Neither source map is mutated. A null incoming map returns the existing map
/// unchanged; when only incoming is present, a new map is returned.
Metadata? mergeMetadata(Metadata? existing, Metadata? incoming) {
  if (incoming == null) {
    return existing;
  }
  if (existing == null) {
    return <String, dynamic>{...incoming};
  }
  return <String, dynamic>{...existing, ...incoming};
}
