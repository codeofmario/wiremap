import cn from 'classnames';
import { useDataTable, DataTableProps } from './DataTable.vm';
import './DataTable.scss';

export const DataTable = (props: DataTableProps) => {
  const { className, columns, rows } = useDataTable(props);

  return (
    <table className={cn('data-table', className)}>
      <thead className="data-table__head">
        <tr>
          {columns.map((col) => (
            <th key={col.key} className="data-table__th">
              {col.label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="data-table__body">
        {rows.map((row, i) => (
          <tr key={i} className="data-table__row">
            {columns.map((col) => (
              <td key={col.key} className="data-table__td">
                {row[col.key]}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
};
