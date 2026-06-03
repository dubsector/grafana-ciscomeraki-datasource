import { DataSourcePlugin } from '@grafana/data';
import { MerakiDS } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { MerakiQuery, MerakiDSOpts } from './types';

export const plugin = new DataSourcePlugin<MerakiDS, MerakiQuery, MerakiDSOpts>(MerakiDS)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
