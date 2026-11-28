import type { Lang, FeatureKey, SolutionKey, Feature, Solution } from '../data/types';
import { features as featuresTr } from '../data/features.tr';
import { features as featuresEn } from '../data/features.en';
import { solutions as solutionsTr } from '../data/solutions.tr';
import { solutions as solutionsEn } from '../data/solutions.en';
import { featurePaths, solutionPaths, staticRoutes, type StaticKey } from '../data/routes';

export const SITE = 'https://messora.tech';

export const abs = (path: string) => SITE + (path === '/' ? '' : path);

/** OpenGraph image path for a page path. */
export function ogFor(path: string): string {
  const key = path === '/' ? 'home' : path.replace(/^\//, '').replace(/\//g, '--');
  return `/og/${key}.png`;
}

export const features = (lang: Lang): Feature[] => (lang === 'tr' ? featuresTr : featuresEn);
export const solutions = (lang: Lang): Solution[] => (lang === 'tr' ? solutionsTr : solutionsEn);
export const feature = (lang: Lang, key: FeatureKey) => features(lang).find((f) => f.key === key)!;
export const solution = (lang: Lang, key: SolutionKey) => solutions(lang).find((s) => s.key === key)!;
export const fPath = (lang: Lang, key: FeatureKey) => featurePaths[key][lang];
export const sPath = (lang: Lang, key: SolutionKey) => solutionPaths[key][lang];
export const r = (lang: Lang, key: StaticKey) => staticRoutes[key][lang];

export function readingMinutes(words: number) {
  return Math.max(1, Math.round(words / 200));
}

export function fmtDate(d: Date, lang: Lang) {
  return new Intl.DateTimeFormat(lang === 'tr' ? 'tr-TR' : 'en-GB', { day: 'numeric', month: 'long', year: 'numeric' }).format(d);
}
