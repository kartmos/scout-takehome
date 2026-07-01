import type { components } from '../../entities/api/__generated__/schema';
import { CLASS_IDS } from '../../entities/photo/classColors';

type Prediction = components['schemas']['Prediction'];

export interface PredictionGroup {
  classId: string;
  count: number;
  minConf: number;
  maxConf: number;
  predictions: Prediction[];
}

export function filterPredictions(
  predictions: Prediction[],
  minConfidence: number | null,
): Prediction[] {
  if (minConfidence === null) return predictions;
  return predictions.filter((p) => p.confidence >= minConfidence);
}

export function groupPredictions(predictions: Prediction[]): PredictionGroup[] {
  const map = new Map<string, Prediction[]>();
  for (const pred of predictions) {
    const arr = map.get(pred.classId);
    if (arr) arr.push(pred);
    else map.set(pred.classId, [pred]);
  }

  const result: PredictionGroup[] = [];
  const knownOrder: readonly string[] = CLASS_IDS;

  for (const classId of knownOrder) {
    const preds = map.get(classId);
    if (preds) {
      const confs = preds.map((p) => Math.round(p.confidence * 100));
      result.push({
        classId,
        count: preds.length,
        minConf: Math.min(...confs),
        maxConf: Math.max(...confs),
        predictions: preds,
      });
      map.delete(classId);
    }
  }

  for (const [classId, preds] of map) {
    const confs = preds.map((p) => Math.round(p.confidence * 100));
    result.push({
      classId,
      count: preds.length,
      minConf: Math.min(...confs),
      maxConf: Math.max(...confs),
      predictions: preds,
    });
  }

  return result;
}
