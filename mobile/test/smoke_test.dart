import 'package:croupier_mobile/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('MainApp 渲染冒烟（M1 起替换为 App 根组件）', (tester) async {
    await tester.pumpWidget(const MainApp());
    expect(find.text('Hello World!'), findsOneWidget);
  });
}
