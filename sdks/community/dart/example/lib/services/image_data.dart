import 'dart:typed_data';

/// Decode an image data URL at both the event and rendering boundaries.
/// Image codecs still validate the bytes and report failures through errorBuilder.
Uint8List? decodeImageDataUrl(String value) {
  try {
    final uri = Uri.parse(value.trim());
    if (uri.scheme != 'data') return null;
    final data = UriData.fromUri(uri);
    if (!data.isBase64 || !data.mimeType.toLowerCase().startsWith('image/')) {
      return null;
    }
    final bytes = data.contentAsBytes();
    return bytes.isEmpty ? null : bytes;
  } on Object {
    return null;
  }
}
