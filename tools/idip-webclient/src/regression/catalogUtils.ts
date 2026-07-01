import type { RegressionCatalogCase } from './catalogTypes';

/** 去重后的 go test 函数名，供 Vite 插件 `-run` 使用 */
export function uniqueGoTests(catalog: RegressionCatalogCase[]): string[] {
  return [...new Set(catalog.map((c) => c.goTest).filter((x): x is string => Boolean(x)))];
}

export function uniqueUnityTests(catalog: RegressionCatalogCase[]): string[] {
  return [...new Set(catalog.map((c) => c.unityTest).filter((x): x is string => Boolean(x)))];
}
