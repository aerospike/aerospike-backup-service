export const toYaml = (data: any): string => {
  const dump = (obj: any, depth = 0): string => {
      if (obj === null || obj === undefined) return '';
      if (typeof obj !== 'object') return String(obj);

      if (Array.isArray(obj)) {
          if (obj.length === 0) return '[]';
          return obj.map(item => `\n${'  '.repeat(depth)}- ${dump(item, depth + 1).trimStart()}`).join('');
      }

      const lines: string[] = [];
      for (const [key, value] of Object.entries(obj)) {
          if (value === undefined || value === null || value === '') continue;

          if (typeof value === 'object' && !Array.isArray(value)) {
              lines.push(`${'  '.repeat(depth)}${key}:\n${dump(value, depth + 1)}`);
          } else {
              lines.push(`${'  '.repeat(depth)}${key}: ${dump(value, depth)}`);
          }
      }
      return lines.join('\n');
  };
  return dump(data).trim();
};