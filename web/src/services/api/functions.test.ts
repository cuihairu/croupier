import { FunctionDescriptor, normalizeFunctionDescriptor } from './functions';

describe('normalizeFunctionDescriptor', () => {
  it('should map input field to inputSchema', () => {
    const raw = {
      id: 'test-function',
      input: '{"type":"object","properties":{"name":{"type":"string"}}}',
      output: '{"type":"object","properties":{"result":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBe(raw.input);
    expect(result.outputSchema).toBe(raw.output);
  });

  it('should prefer inputSchema over input', () => {
    const raw = {
      id: 'test-function',
      inputSchema: '{"type":"object"}',
      input: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBe(raw.inputSchema);
  });

  it('should prefer input_schema over input', () => {
    const raw = {
      id: 'test-function',
      inputSchema: '{"type":"object"}',
      input: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBe(raw.inputSchema);
  });

  it('should map output field to outputSchema', () => {
    const raw = {
      id: 'test-function',
      output: '{"type":"object","properties":{"data":{"type":"array"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.outputSchema).toBe(raw.output);
  });

  it('should prefer outputSchema over output', () => {
    const raw = {
      id: 'test-function',
      outputSchema: '{"type":"object"}',
      output: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.outputSchema).toBe(raw.outputSchema);
  });

  it('should prefer outputSchema over output', () => {
    const raw = {
      id: 'test-function',
      outputSchema: '{"type":"object"}',
      output: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.outputSchema).toBe(raw.outputSchema);
  });

  it('should handle undefined input/output', () => {
    const raw = {
      id: 'test-function',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBeUndefined();
    expect(result.outputSchema).toBeUndefined();
  });

  it('should normalize displayName from string', () => {
    const raw = {
      id: 'test-function',
      displayName: 'Test Function',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.displayName).toEqual({ en: 'Test Function', zh: 'Test Function' });
  });

  it('should normalize displayName from object', () => {
    const raw = {
      id: 'test-function',
      displayName: { en: 'Test Function', zh: '测试函数' },
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.displayName).toEqual({ en: 'Test Function', zh: '测试函数' });
  });

  it('should normalize displayName string to LocalizedText', () => {
    const raw = {
      id: 'test-function',
      displayName: 'Test Function',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.displayName).toEqual({ en: 'Test Function', zh: 'Test Function' });
  });
});
