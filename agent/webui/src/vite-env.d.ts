/// <reference types="vite/client" />

// 窄声明：仅测试用 readFileSync 读源码做挂载断言（仓库不引入 @types/node）
declare module "node:fs" {
  export function readFileSync(path: string | URL, encoding: string): string;
}
