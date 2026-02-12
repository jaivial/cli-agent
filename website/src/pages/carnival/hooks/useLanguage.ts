import { useState, useCallback } from 'react';
import { atom, useAtom } from 'jotai';
import type { Language } from '../types';

const languageAtom = atom<Language['code']>('pt');

export function useLanguage() {
  const [language, setLanguage] = useAtom(languageAtom);

  const toggleLanguage = useCallback(() => {
    setLanguage((prev) => (prev === 'pt' ? 'es' : 'pt'));
  }, [setLanguage]);

  return { language, toggleLanguage };
}
