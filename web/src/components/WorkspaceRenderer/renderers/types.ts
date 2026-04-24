export interface RendererCommonProps {
  objectKey: string;
  context?: Record<string, any>;
}

export type RendererProps<TLayout> = RendererCommonProps & {
  layout: TLayout;
};
