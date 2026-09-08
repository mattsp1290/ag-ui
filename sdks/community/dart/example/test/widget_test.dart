import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

import 'package:ag_ui_example/main.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/pages/chat_page.dart';

void main() {
  setUp(() => WidgetController.hitTestWarningShouldBeFatal = true);
  tearDown(() => WidgetController.hitTestWarningShouldBeFatal = false);

  testWidgets('invalid base URL renders an actionable configuration error', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const MyApp(baseUrlForValidation: 'ftp://wrong'));
    expect(find.byKey(const Key('configuration-error')), findsOneWidget);
    expect(find.textContaining('AG_UI_BASE_URL'), findsOneWidget);
    expect(find.textContaining('http://127.0.0.1:8080'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  for (final size in const [Size(1400, 1000), Size(1400, 600)]) {
    testWidgets('desktop navigation reaches every destination at $size', (
      WidgetTester tester,
    ) async {
      _setSurface(tester, size);
      await tester.pumpWidget(const MyApp());
      await tester.pump();

      expect(find.byKey(const Key('desktop-navigation')), findsOneWidget);
      for (final endpoint in EndpointConfig.availableEndpoints) {
        final destination = find.byKey(ValueKey('nav-${endpoint.path}'));
        await tester.ensureVisible(destination);
        await tester.tap(destination);
        await tester.pump();
        expect(
          tester.takeException(),
          isNull,
          reason: 'building ${endpoint.name} at $size',
        );
        expect(
          find.byKey(ValueKey(endpoint.path)),
          findsOneWidget,
          reason: '${endpoint.name} should be reachable at $size',
        );
      }
    });
  }

  testWidgets('narrow navigation drawer reaches every destination', (
    WidgetTester tester,
  ) async {
    const size = Size(390, 600);
    _setSurface(tester, size);
    await tester.pumpWidget(const MyApp());
    await tester.pump();

    expect(find.byKey(const Key('open-navigation-menu')), findsOneWidget);
    for (final endpoint in EndpointConfig.availableEndpoints) {
      await tester.tap(find.byKey(const Key('open-navigation-menu')));
      await tester.pumpAndSettle();
      final destination = find.byKey(ValueKey('drawer-nav-${endpoint.path}'));
      expect(destination, findsOneWidget);
      await tester.ensureVisible(destination);
      await tester.tap(destination);
      await tester.pumpAndSettle();
      expect(
        tester.takeException(),
        isNull,
        reason: 'building ${endpoint.name} at $size',
      );
      expect(
        find.byKey(ValueKey(endpoint.path)),
        findsOneWidget,
        reason: '${endpoint.name} should be reachable at $size',
      );
    }
  });

  testWidgets('narrow drawer dismisses with Escape', (tester) async {
    _setSurface(tester, const Size(390, 600));
    await tester.pumpWidget(const MyApp());

    await tester.tap(find.byKey(const Key('open-navigation-menu')));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('drawer-nav-agentic_chat')),
      findsOneWidget,
    );

    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('drawer-nav-agentic_chat')), findsNothing);
  });

  testWidgets('drawer does not reopen after narrow-wide-narrow resize', (
    tester,
  ) async {
    _setSurface(tester, const Size(390, 600));
    await tester.pumpWidget(const MyApp());
    await tester.tap(find.byKey(const Key('open-navigation-menu')));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('drawer-nav-agentic_chat')),
      findsOneWidget,
    );

    tester.view.physicalSize = const Size(1400, 600);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('desktop-navigation')), findsOneWidget);

    tester.view.physicalSize = const Size(390, 600);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('open-navigation-menu')), findsOneWidget);
    expect(find.byKey(const ValueKey('drawer-nav-agentic_chat')), findsNothing);
  });

  testWidgets('switching chat endpoints disposes the old page state', (
    tester,
  ) async {
    _setSurface(tester, const Size(1400, 600));
    await tester.pumpWidget(const MyApp());

    final imageGen = find.byKey(const ValueKey('nav-image-gen'));
    await tester.ensureVisible(imageGen);
    await tester.tap(imageGen);
    await tester.pumpAndSettle();
    final oldState = tester
        .element(find.byType(ChatPageView))
        .read<ChatPageState>();
    expect(oldState.disposed, isFalse);

    final reasoning = find.byKey(const ValueKey('nav-reasoning'));
    await tester.ensureVisible(reasoning);
    await tester.tap(reasoning);
    await tester.pumpAndSettle();

    expect(oldState.disposed, isTrue);
    expect(
      tester.element(find.byType(ChatPageView)).read<ChatPageState>(),
      isNot(same(oldState)),
    );
  });
}

void _setSurface(WidgetTester tester, Size size) {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}
