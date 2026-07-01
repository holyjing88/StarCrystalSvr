/** 回归面板展示用例（三类回归共用列） */
export interface RegressionCatalogCase {
  id: string;
  api: string;
  servicePurpose: string;
  verify: string;
  docRef?: string;
  /** server-go `go test` 函数名 */
  goTest?: string;
  /** Unity NUnit 方法名片段 */
  unityTest?: string;
}

export interface RegressionRowResult {
  id: string;
  passed: boolean;
  durationMs?: number;
  error?: string;
  skipped?: boolean;
}
