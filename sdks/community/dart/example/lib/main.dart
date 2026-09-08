import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'models/endpoint_config.dart';
import 'pages/chat_page.dart';
import 'pages/multimodal_chat_page.dart';
import 'pages/client_tools_page.dart';
import 'pages/live_state_page.dart';
import 'services/ag_ui_service.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  final String? baseUrlOverride;

  const MyApp({super.key, this.baseUrlOverride});

  @override
  Widget build(BuildContext context) {
    try {
      AgUiService.resolveBaseUrl(baseUrlOverride);
    } on ArgumentError catch (error) {
      return MaterialApp(
        title: 'AG-UI Flutter Dojo',
        home: Scaffold(
          key: const Key('configuration-error'),
          body: Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: SelectableText(
                'Cannot start the AG-UI Flutter Dojo.\n\n'
                'AG_UI_BASE_URL must be a valid HTTP or HTTPS URL, for example '
                'http://127.0.0.1:8080.\n\n$error',
                textAlign: TextAlign.center,
              ),
            ),
          ),
        ),
      );
    }
    return ChangeNotifierProvider(
      create: (context) => ChatAppState(),
      child: MaterialApp(
        title: 'AG-UI Flutter Dojo',
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
          useMaterial3: true,
        ),
        home: MyHomePage(),
      ),
    );
  }
}

class ChatAppState extends ChangeNotifier {
  int _selectedEndpointIndex = 0;
  final List<EndpointConfig> endpoints = EndpointConfig.availableEndpoints;

  int get selectedEndpointIndex => _selectedEndpointIndex;
  EndpointConfig get selectedEndpoint => endpoints[_selectedEndpointIndex];

  void selectEndpoint(int index) {
    if (index >= 0 && index < endpoints.length) {
      _selectedEndpointIndex = index;
      notifyListeners();
    }
  }
}

class MyHomePage extends StatefulWidget {
  @override
  State<MyHomePage> createState() => _MyHomePageState();
}

class _MyHomePageState extends State<MyHomePage> {
  bool _menuOpen = false;

  @override
  Widget build(BuildContext context) {
    final appState = context.watch<ChatAppState>();
    final selectedIndex = appState.selectedEndpointIndex;
    final endpoint = appState.selectedEndpoint;

    final pageKey = ValueKey(endpoint.path);
    Widget page;
    switch (endpoint.featureKind) {
      case FeatureKind.multimodal:
        page = MultimodalChatPage(key: pageKey, endpoint: endpoint);
      case FeatureKind.clientTools:
      case FeatureKind.approval:
        page = ClientToolsPage(key: pageKey, endpoint: endpoint);
      case FeatureKind.liveState:
        page = LiveStatePage(key: pageKey, endpoint: endpoint);
      case FeatureKind.chat:
        page = ChatPage(key: pageKey, endpoint: endpoint);
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        final narrow = constraints.maxWidth < 600;
        final drawer = _NavigationDrawer(
          endpoints: appState.endpoints,
          selectedIndex: selectedIndex,
          onDestinationSelected: appState.selectEndpoint,
          onClose: () => setState(() => _menuOpen = false),
        );

        if (narrow) {
          return Scaffold(
            body: Stack(
              children: [
                Positioned.fill(child: page),
                if (_menuOpen)
                  Positioned.fill(
                    child: GestureDetector(
                      onTap: () => setState(() => _menuOpen = false),
                      child: ColoredBox(
                        color: Colors.black26,
                        child: Align(
                          alignment: Alignment.topRight,
                          child: Material(
                            elevation: 8,
                            child: SizedBox(
                              width: 320,
                              height: constraints.maxHeight,
                              child: drawer,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                Positioned.fill(
                  child: SafeArea(
                    child: Align(
                      alignment: Alignment.topRight,
                      child: Builder(
                        builder: (context) => IconButton(
                          key: const Key('open-navigation-menu'),
                          tooltip: 'Open navigation menu',
                          icon: const Icon(Icons.menu),
                          onPressed: () => setState(() => _menuOpen = true),
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          );
        }

        return Scaffold(
          body: Row(
            children: [
              SafeArea(
                child: _DesktopNavigation(
                  endpoints: appState.endpoints,
                  selectedIndex: selectedIndex,
                  onDestinationSelected: appState.selectEndpoint,
                ),
              ),
              Expanded(child: page),
            ],
          ),
        );
      },
    );
  }
}

class _DesktopNavigation extends StatelessWidget {
  final List<EndpointConfig> endpoints;
  final int selectedIndex;
  final ValueChanged<int> onDestinationSelected;

  const _DesktopNavigation({
    required this.endpoints,
    required this.selectedIndex,
    required this.onDestinationSelected,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      key: const Key('desktop-navigation'),
      width: 256,
      child: ListView.builder(
        padding: const EdgeInsets.symmetric(vertical: 12),
        itemCount: endpoints.length,
        itemBuilder: (context, index) {
          final endpoint = endpoints[index];
          return ListTile(
            key: ValueKey('nav-${endpoint.path}'),
            leading: Icon(endpoint.icon),
            title: Text(endpoint.name),
            selected: index == selectedIndex,
            onTap: () => onDestinationSelected(index),
          );
        },
      ),
    );
  }
}

class _NavigationDrawer extends StatelessWidget {
  final List<EndpointConfig> endpoints;
  final int selectedIndex;
  final ValueChanged<int> onDestinationSelected;
  final VoidCallback onClose;

  const _NavigationDrawer({
    required this.endpoints,
    required this.selectedIndex,
    required this.onDestinationSelected,
    required this.onClose,
  });

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: SingleChildScrollView(
        padding: const EdgeInsets.symmetric(vertical: 12),
        child: Column(
          children: [
            const Padding(
              padding: EdgeInsets.fromLTRB(28, 8, 20, 16),
              child: Text(
                'AG-UI Flutter Dojo',
                style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
              ),
            ),
            for (var index = 0; index < endpoints.length; index++)
              ListTile(
                key: ValueKey('drawer-nav-${endpoints[index].path}'),
                leading: Icon(endpoints[index].icon),
                title: Text(endpoints[index].name),
                selected: index == selectedIndex,
                onTap: () {
                  onDestinationSelected(index);
                  onClose();
                },
              ),
          ],
        ),
      ),
    );
  }
}
