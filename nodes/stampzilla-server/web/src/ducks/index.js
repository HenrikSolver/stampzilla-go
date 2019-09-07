import { combineReducers } from 'redux-immutable';

import app from './app';
import certificates from './certificates';
import connection from './connection';
import connections from './connections';
import devices from './devices';
import nodes from './nodes';
import persons from './persons';
import requests from './requests';
import rules from './rules';
import savedstates from './savedstates';
import schedules from './schedules';
import server from './server';

const rootReducer = combineReducers({
  app,
  certificates,
  connection,
  connections,
  devices,
  nodes,
  persons,
  requests,
  rules,
  savedstates,
  schedules,
  server,
});

export default rootReducer;
