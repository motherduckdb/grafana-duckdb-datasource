import React from 'react';

type JSXType = React.ElementType;
type JSXProps = Record<string, unknown> & { key?: React.Key };

export const Fragment = React.Fragment;

function createCompatElement(type: JSXType, props: JSXProps | null, key?: React.Key): React.ReactElement {
  const config = props == null ? {} : { ...props };
  if (key !== undefined) {
    config.key = key;
  }

  return React.createElement(type, config);
}

export function jsx(type: JSXType, props: JSXProps, key?: React.Key): React.ReactElement {
  return createCompatElement(type, props, key);
}

export const jsxs = jsx;

export function jsxDEV(type: JSXType, props: JSXProps, key?: React.Key): React.ReactElement {
  return createCompatElement(type, props, key);
}
