import 'dart:async';

/// A lazy, single-subscription transform with immediate cancellation even when
/// no output has yet been produced. Holds one input iterator, never a queue of
/// expanded lines/messages. Synchronous delivery lets downstream pause between
/// outputs from the same input chunk.
Stream<R> sseTransform<T, R>(
  Stream<T> source,
  Iterable<R> Function(T) convert, {
  required Iterable<R> Function() finish,
  required void Function() clear,
}) {
  late StreamController<R> controller;
  StreamSubscription<T>? subscription;
  Iterator<R>? pending;
  Future<void>? cancellation;
  var stopped = false;
  var sourceDone = false;
  var draining = false;

  Future<void> cancel() {
    stopped = true;
    pending = null;
    clear();
    return cancellation ??= Future<void>.sync(() => subscription?.cancel());
  }

  void fail(Object error, StackTrace stack) {
    if (stopped) {
      return;
    }
    // Clear/cancel before reporting the error; no partial EOF flush follows.
    final cancelled = cancel().then<void>(
      (_) {},
      onError: (Object _, StackTrace __) {
        // A secondary cleanup failure must not escape the primary stream error.
      },
    );
    // Automatic controller closure invokes onCancel too. Reuse the handled
    // future there; explicit cancellation outside fail still returns its error.
    cancellation = cancelled;
    controller.addError(error, stack);
    unawaited(cancelled.then((_) => controller.close()));
  }

  void drain() {
    if (draining || stopped) {
      return;
    }
    draining = true;
    try {
      while (!stopped && !controller.isPaused && pending != null) {
        final iterator = pending!;
        if (iterator.moveNext()) {
          controller.add(iterator.current);
        } else {
          pending = null;
        }
      }
      if (!stopped && pending == null) {
        if (sourceDone) {
          stopped = true;
          clear();
          unawaited(controller.close());
        } else if (!controller.isPaused) {
          subscription?.resume();
        }
      }
    } on Object catch (error, stack) {
      fail(error, stack);
    } finally {
      draining = false;
    }
  }

  controller = StreamController<R>(
    sync: true,
    onListen: () {
      subscription = source.listen(
        (input) {
          subscription!.pause();
          try {
            pending = convert(input).iterator;
            drain();
          } on Object catch (error, stack) {
            fail(error, stack);
          }
        },
        onError: fail,
        onDone: () {
          if (stopped) {
            return;
          }
          sourceDone = true;
          try {
            pending = finish().iterator;
            drain();
          } on Object catch (error, stack) {
            fail(error, stack);
          }
        },
      );
    },
    onPause: () {
      // One pause per input already applies while draining a pending iterator.
      if (pending == null) {
        subscription?.pause();
      }
    },
    onResume: drain,
    onCancel: cancel,
  );
  return controller.stream;
}
