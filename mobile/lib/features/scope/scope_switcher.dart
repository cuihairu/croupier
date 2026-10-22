/// 顶栏 scope 切换器（设计稿 §3.3）：底部弹层两段选择 game → env。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'scope_controller.dart';

class ScopeSwitcher extends ConsumerStatefulWidget {
  const ScopeSwitcher({super.key});

  @override
  ConsumerState<ScopeSwitcher> createState() => _ScopeSwitcherState();
}

class _ScopeSwitcherState extends ConsumerState<ScopeSwitcher> {
  @override
  void initState() {
    super.initState();
    // 进入已登录壳即预取（幂等；切换器打开时可复用结果）。
    Future.microtask(() => ref.read(scopeControllerProvider.notifier).load());
  }

  Future<void> _openSheet() async {
    ref.read(scopeControllerProvider.notifier).load();
    await showModalBottomSheet<void>(
      context: context,
      builder: (sheetContext) => const ScopeSheet(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(scopeControllerProvider);
    final selected = state.selected;
    final label = selected == null
        ? '选择 scope'
        : '${selected.gameId} / ${selected.env}';
    return TextButton(
      key: const ValueKey('scope-switcher'),
      onPressed: _openSheet,
      child: Text(label, style: const TextStyle(color: Colors.white)),
    );
  }
}

class ScopeSheet extends ConsumerWidget {
  const ScopeSheet({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(scopeControllerProvider);
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('选择游戏与环境', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            if (state.loading)
              const Padding(
                padding: EdgeInsets.all(24),
                child: Center(child: CircularProgressIndicator()),
              )
            else if (state.error != null) ...[
              Text(
                state.error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
              const SizedBox(height: 8),
              TextButton(
                onPressed: () =>
                    ref.read(scopeControllerProvider.notifier).load(),
                child: const Text('重试'),
              ),
            ] else if (state.games.isEmpty)
              const Padding(
                padding: EdgeInsets.all(24),
                child: Center(child: Text('暂无可选游戏')),
              )
            else
              Flexible(
                child: ListView(
                  shrinkWrap: true,
                  children: [
                    for (final game in state.games)
                      ExpansionTile(
                        key: ValueKey('scope-game-${game.gameId}'),
                        title: Text(game.gameName),
                        subtitle: Text(game.gameId),
                        initiallyExpanded:
                            state.selected?.gameId == game.gameId,
                        children: [
                          for (final env in game.envs)
                            ListTile(
                              key: ValueKey('scope-env-${game.gameId}-$env'),
                              title: Text(env),
                              trailing:
                                  state.selected?.gameId == game.gameId &&
                                      state.selected?.env == env
                                  ? const Icon(Icons.check)
                                  : null,
                              onTap: () async {
                                await ref
                                    .read(scopeControllerProvider.notifier)
                                    .select(game.gameId, env);
                                if (context.mounted) {
                                  Navigator.of(context).pop();
                                }
                              },
                            ),
                        ],
                      ),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}
