import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:ag_ui_example/main.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';

void main() {
  testWidgets('invalid base URL renders an actionable configuration error', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const MyApp(baseUrlOverride: 'ftp://wrong'));
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
      await tester.pump(const Duration(seconds: 1));
      final destination = find.byKey(ValueKey('drawer-nav-${endpoint.path}'));
      expect(destination, findsOneWidget);
      await tester.ensureVisible(destination);
      await tester.tap(destination);
      await tester.pump(const Duration(seconds: 1));
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

void _setSurface(WidgetTester tester, Size size) {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}
