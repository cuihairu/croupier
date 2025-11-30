import React, { useEffect, useMemo, useState } from "react";
import { AppstoreOutlined, ThunderboltOutlined } from "@ant-design/icons";
import { Empty, Select, Spin } from "antd";
import classNames from "classnames";
import { listMyGames, type Game, type GameEnvMeta } from "@/services/croupier";
import styles from "./index.less";

type GameSelectorProps = {
  value?: string;
  envValue?: string;
  onChange?: (gameId?: string) => void;
  onEnvChange?: (env?: string) => void;
  className?: string;
};

type EnvOption = {
  value: string;
  label: string;
  color?: string;
  description?: string;
};

const DEFAULT_ENV_OPTIONS: EnvOption[] = [
  { value: "prod", label: "PROD", color: "#13c2c2", description: "Production" },
  { value: "stage", label: "STAGE", color: "#fa8c16", description: "Staging" },
  { value: "test", label: "TEST", color: "#722ed1", description: "Testing" },
  { value: "dev", label: "DEV", color: "#1677ff", description: "Development" },
];

const FALLBACK_COLOR = "#8c8c8c";

const buildEnvOptions = (game?: Game): EnvOption[] => {
  if (!game) return DEFAULT_ENV_OPTIONS;
  const fromMeta =
    Array.isArray(game.envMeta) && game.envMeta.length > 0
      ? game.envMeta
      : Array.isArray(game.envs) && game.envs.length > 0
        ? game.envs.map((env) => ({ env }))
        : null;

  const source = (fromMeta ?? DEFAULT_ENV_OPTIONS) as (
    | GameEnvMeta
    | EnvOption
  )[];
  return source
    .map((env) => {
      const name = (env as GameEnvMeta).env || (env as EnvOption).value;
      if (!name) return undefined;
      const fallback = DEFAULT_ENV_OPTIONS.find((opt) => opt.value === name);
      return {
        value: name,
        label: name.toUpperCase(),
        color:
          (env as GameEnvMeta).color ||
          (env as EnvOption).color ||
          fallback?.color,
        description:
          (env as GameEnvMeta).description ||
          (env as EnvOption).description ||
          fallback?.description,
      } as EnvOption;
    })
    .filter((env): env is EnvOption => Boolean(env?.value));
};

const toRGBA = (hex?: string, alpha = 0.16) => {
  if (!hex || !hex.startsWith("#")) return undefined;
  const raw = hex.replace("#", "");
  const normalized =
    raw.length === 3
      ? raw
          .split("")
          .map((char) => char + char)
          .join("")
      : raw;
  const int = parseInt(normalized, 16);
  const r = (int >> 16) & 255;
  const g = (int >> 8) & 255;
  const b = int & 255;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
};

const GameSelector: React.FC<GameSelectorProps> = ({
  value,
  envValue,
  onChange,
  onEnvChange,
  className,
}) => {
  const canListGames = true;

  const [games, setGames] = useState<Game[]>([]);
  const [loading, setLoading] = useState(false);

  const isGameControlled = typeof value !== "undefined";
  const [gameState, setGameState] = useState<string | undefined>(
    () => value ?? localStorage.getItem("game_id") ?? undefined,
  );

  const isEnvControlled = typeof envValue !== "undefined";
  const [envState, setEnvState] = useState<string | undefined>(
    () => envValue ?? localStorage.getItem("env") ?? undefined,
  );

  useEffect(() => {
    if (isGameControlled) {
      setGameState(value);
    }
  }, [isGameControlled, value]);

  useEffect(() => {
    if (isEnvControlled) {
      setEnvState(envValue);
    }
  }, [envValue, isEnvControlled]);

  useEffect(() => {
    if (!canListGames) return;
    let mounted = true;
    setLoading(true);
    listMyGames()
      .then((res) => {
        if (!mounted) return;
        const scopedGames = Array.isArray(res?.games) ? res.games : [];
        setGames(scopedGames);
      })
      .catch((err) => {
        console.error("Failed to load games", err);
      })
      .finally(() => {
        if (mounted) {
          setLoading(false);
        }
      });
    return () => {
      mounted = false;
    };
  }, [canListGames]);

  const activeGame = useMemo(() => {
    if (!games.length) return undefined;
    const match = games.find((g) => g.name === gameState);
    return match || games[0];
  }, [games, gameState]);

  const envOptions = useMemo(() => buildEnvOptions(activeGame), [activeGame]);

  useEffect(() => {
    if (!games.length) {
      return;
    }
    if (!gameState || !games.some((g) => g.name === gameState)) {
      const fallback = games[0]?.name;
      if (fallback) {
        if (!isGameControlled) {
          setGameState(fallback);
          localStorage.setItem("game_id", fallback);
        }
        onChange?.(fallback);
      }
    }
  }, [games, gameState, isGameControlled, onChange]);

  useEffect(() => {
    if (!envOptions.length) {
      return;
    }
    const currentEnv = isEnvControlled ? envValue : envState;
    if (!currentEnv || !envOptions.some((env) => env.value === currentEnv)) {
      const fallback = envOptions[0]?.value;
      if (fallback) {
        if (!isEnvControlled) {
          setEnvState(fallback);
          localStorage.setItem("env", fallback);
        }
        onEnvChange?.(fallback);
      }
    }
  }, [envOptions, envState, envValue, isEnvControlled, onEnvChange]);

  const handleGameChange = (next?: string) => {
    if (!next) {
      return;
    }
    if (!isGameControlled) {
      setGameState(next);
      localStorage.setItem("game_id", next);
    }
    onChange?.(next);
  };

  const handleEnvChange = (next?: string) => {
    if (!next) {
      return;
    }
    if (!isEnvControlled) {
      setEnvState(next);
      localStorage.setItem("env", next);
    }
    onEnvChange?.(next);
  };

  const currentEnv =
    (isEnvControlled ? envValue : envState) ?? envOptions[0]?.value;
  const colorDot = (color?: string) => ({
    backgroundColor: color || FALLBACK_COLOR,
  });

  const selectOptions = (games || []).map((game) => {
    const alias = game.alias_name || game.display_name || game.name;
    return {
      value: game.name,
      label: (
        <div className={styles.gameOption}>
          <span className={styles.colorDot} style={colorDot(game.color)} />
          <div className={styles.gameTexts}>
            <span className={styles.gameAlias}>{alias}</span>
            <span className={styles.gameId}>{game.name}</span>
          </div>
        </div>
      ),
      searchValue: `${game.name} ${alias}`.toLowerCase(),
    };
  });

  const envSelectOptions = envOptions.map((env) => ({
    value: env.value,
    label: (
      <div className={styles.gameOption}>
        <span className={styles.colorDot} style={colorDot(env.color)} />
        <span className={styles.gameAlias}>{env.label}</span>
      </div>
    ),
    searchValue: `${env.value} ${env.label}`.toLowerCase(),
  }));

  return (
    <div className={classNames(styles.scopeInline, className)}>
      <div className={styles.inlineGroup}>
        <span className={styles.inlineLabel}>
          <AppstoreOutlined className={styles.inlineLabelIcon} />
          游戏
        </span>
        {loading ? (
          <span className={styles.spinner}>
            <Spin size="small" /> 加载中
          </span>
        ) : games.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="暂无游戏"
            style={{ margin: 0 }}
          />
        ) : (
          <Select
            className={styles.gameSelect}
            value={gameState}
            placeholder="选择游戏"
            onChange={(val) => handleGameChange(val as string)}
            options={selectOptions}
            showSearch
            optionFilterProp="searchValue"
            popupMatchSelectWidth={360}
            size="middle"
          />
        )}
      </div>

      <div className={styles.inlineGroup}>
        <span className={styles.inlineLabel}>
          <ThunderboltOutlined className={styles.inlineLabelIcon} />
          环境
        </span>
        {envOptions.length === 0 ? (
          <span className={styles.emptyHint}>未配置可用环境</span>
        ) : (
          <Select
            className={styles.envSelect}
            value={currentEnv}
            placeholder="选择环境"
            onChange={(val) => handleEnvChange(val as string)}
            options={envSelectOptions as any}
            showSearch
            optionFilterProp="searchValue"
            size="middle"
          />
        )}
      </div>
    </div>
  );
};

export default GameSelector;
