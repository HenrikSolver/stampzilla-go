import { Map, fromJS } from 'immutable';
import { defineAction } from 'redux-define';
import makeUUID from 'uuid/v4';

const c = defineAction(
  'rules',
  ['ADD', 'SAVE', 'UPDATE', 'UPDATE_STATE'],
);

const defaultState = Map({
  list: Map(),
  state: Map(),
});

// Actions
export function add(rule) {
  return { type: c.ADD, rule };
}
export function save(rule) {
  return { type: c.SAVE, rule };
}
export function update(rules) {
  return { type: c.UPDATE, rules };
}
export function updateState(rules) {
  return { type: c.UPDATE_STATE, rules };
}

// Subscribe to channels and register the action for the packages
export function subscribe(dispatch) {
  return {
    rules: rules => dispatch(update(rules)),
    server: ({ rules }) => rules && dispatch(updateState(rules)),
  };
}

// Reducer
export default function reducer(state = defaultState, action) {
  switch (action.type) {
    case c.ADD: {
      const rule = {
        ...action.rule,
        uuid: makeUUID(),
      };
      return state
        .setIn(['list', rule.uuid], fromJS(rule));
    }
    case c.SAVE: {
      return state
        .mergeIn(['list', action.rule.uuid], fromJS(action.rule));
    }
    case c.UPDATE: {
      return state
        .set('list', fromJS(action.rules));
    }
    case c.UPDATE_STATE: {
      return state
        .set('state', fromJS(action.rules));
    }
    default: return state;
  }
}
