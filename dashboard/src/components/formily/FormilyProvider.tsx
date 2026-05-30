import React from 'react';
import type { Form } from '@formily/core';
import { FormProvider } from '@formily/react';

interface FormilyProviderProps {
  form: Form;
  children: React.ReactNode;
}

export default function FormilyProvider({ form, children }: FormilyProviderProps) {
  return <FormProvider form={form}>{children}</FormProvider>;
}
