import 'package:couplelink/core/connection.dart';
import 'package:couplelink/core/device_feedback.dart';
import 'package:couplelink/core/remote_interactions.dart';
import 'package:couplelink/core/state.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class InteractionFixture extends ConnectionController {
  final pending = <Map<String, dynamic>>[];
  final results = <Map<String, dynamic>>[];
  final sends = <Map<String, dynamic>>[];
  @override
  ConnectionState build() => const ConnectionState(
    origin: 'https://example.com',
    user: {'id': 'alice'},
    couple: {'id': 'couple'},
  );
  @override
  Future<Map<String, dynamic>> request(
    String method,
    String path, {
    Map<String, dynamic>? body,
    bool auth = true,
    int responseLimit = 256 * 1024,
  }) async {
    if (path.startsWith('/interactions?')) {
      return {'interactions': <Map<String, dynamic>>[]};
    }
    if (path == '/interactions/claim') {
      return pending.isEmpty
          ? {'interaction': null}
          : {'interaction': pending.removeAt(0), 'remaining_ms': 30000};
    }
    if (path == '/interactions/result') {
      results.add(body!);
      return {'ok': true};
    }
    if (path == '/interactions') {
      sends.add(body!);
      return {'status': 'sent'};
    }
    throw StateError('Unexpected request');
  }
}

class FakeFeedback implements DeviceFeedback {
  final calls = <bool>[];
  @override
  Future<FeedbackOutcome> preview({required bool sound}) async {
    calls.add(sound);
    return FeedbackOutcome.requested;
  }

  @override
  Future<void> stop() async {}
}

Future<void> until(bool Function() ready) async {
  for (var i = 0; i < 200; i++) {
    if (ready()) return;
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
  fail('Interaction did not complete');
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('ring is muted by default; vibration follows recipient setting and reports actual request', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance(),
        api = InteractionFixture(),
        feedback = FakeFeedback();
    api.pending.add({'id': 'ring-id', 'action': 'ring'});
    final container = ProviderContainer(
      overrides: [
        preferencesProvider.overrideWithValue(prefs),
        connectionProvider.overrideWith(() => api),
        deviceFeedbackProvider.overrideWithValue(feedback),
      ],
    );
    container.read(interactionProvider);
    try {
      await until(
        () =>
            container.read(interactionProvider).incoming != null &&
            api.results.isNotEmpty,
      );
      expect(feedback.calls, isEmpty);
      expect(api.results.single['result'], 'suppressed');
      container.read(interactionProvider.notifier).dismiss();
      api.pending.add({'id': 'vibration-id', 'action': 'vibrate'});
      await container.read(interactionProvider.notifier).sync();
      await until(() => api.results.length == 2);
      expect(feedback.calls, [false]);
      expect(api.results.last['result'], 'requested');
      await container.read(interactionProvider.notifier).send('poke');
      expect(api.sends.single['action'], 'poke');
    } finally {
      container.dispose();
    }
  });
}
