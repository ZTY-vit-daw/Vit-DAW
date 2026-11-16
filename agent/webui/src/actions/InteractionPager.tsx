// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// InteractionPager 交互分页器。

import type { JsonRecord } from "../types";
import { clampNumber, firstArray, textValue } from "../lib/util";

type InteractionPage = {
  reviewItems: JsonRecord[];
  fields: JsonRecord[];
  showCommand: boolean;
};

export function emptyInteractionPage(): InteractionPage {
  return { reviewItems: [], fields: [], showCommand: false };
}

export function buildInteractionPages({
  reviewItems,
  fields,
  command
}: {
  reviewItems: JsonRecord[];
  fields: JsonRecord[];
  command: JsonRecord;
}): InteractionPage[] {
  const hasCommand = Object.keys(command).length > 0;
  const commandWeight = hasCommand ? 2 : 0;
  const totalWeight =
    reviewItems.length +
    commandWeight +
    fields.reduce((sum, field) => sum + interactionFieldWeight(field), 0);
  if (totalWeight <= 6) {
    return [{ reviewItems, fields, showCommand: hasCommand }];
  }

  const pages: InteractionPage[] = [];
  let current = emptyInteractionPage();
  let weight = 0;
  const flush = () => {
    if (current.reviewItems.length > 0 || current.fields.length > 0 || current.showCommand) {
      pages.push(current);
      current = emptyInteractionPage();
      weight = 0;
    }
  };
  const reserve = (nextWeight: number) => {
    if (weight > 0 && weight + nextWeight > 6) {
      flush();
    }
    weight += nextWeight;
  };

  reviewItems.forEach((item) => {
    reserve(1);
    current.reviewItems.push(item);
  });
  fields.forEach((field) => {
    const fieldWeight = interactionFieldWeight(field);
    reserve(fieldWeight);
    current.fields.push(field);
  });
  if (hasCommand) {
    reserve(commandWeight);
    current.showCommand = true;
  }
  flush();
  return pages.length > 0 ? pages : [emptyInteractionPage()];
}

function interactionFieldWeight(field: JsonRecord): number {
  const kind = textValue(field.kind ?? field.type, "text").toLowerCase();
  const optionCount = firstArray(field.options).length;
  if (kind === "choice" || kind === "select") {
    return Math.max(2, Math.min(5, 1 + Math.ceil(optionCount / 2)));
  }
  if (kind === "textarea" || kind === "long_text" || kind === "multiline") {
    return 2;
  }
  return 1.25;
}

export function InteractionPager({
  pageIndex,
  pageCount,
  onPageChange
}: {
  pageIndex: number;
  pageCount: number;
  onPageChange: (pageIndex: number) => void;
}) {
  const safePage = clampNumber(pageIndex, 0, Math.max(0, pageCount - 1));
  return (
    <div className="interaction-pager">
      <button type="button" disabled={safePage <= 0} onClick={() => onPageChange(safePage - 1)}>
        上一页      </button>
      <span>{safePage + 1} / {pageCount}</span>
      <button type="button" disabled={safePage >= pageCount - 1} onClick={() => onPageChange(safePage + 1)}>
        下一页      </button>
    </div>
  );
}
