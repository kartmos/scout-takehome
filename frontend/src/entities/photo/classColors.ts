export const CLASS_IDS = [
  'powdery_mildew',
  'mirid',
  'whitefly_aphid',
  'miner_tuta',
  'thrips',
  'spider_mites',
] as const;

export type ClassIdKey = (typeof CLASS_IDS)[number];

export const CLASS_COLORS: Record<string, string> = {
  powdery_mildew: '#FF00A8',
  mirid: '#0057FF',
  whitefly_aphid: '#00D9FF',
  miner_tuta: '#FF6B00',
  thrips: '#FF1744',
  spider_mites: '#8A2BE2',
};

export const CLASS_LABELS: Record<string, string> = {
  powdery_mildew: 'Powdery Mildew',
  mirid: 'Mirid',
  whitefly_aphid: 'Whitefly / Aphid',
  miner_tuta: 'Leaf Miner / Tuta',
  thrips: 'Thrips',
  spider_mites: 'Spider Mites',
};

export const DEFAULT_CLASS_COLOR = '#AAAAAA';
