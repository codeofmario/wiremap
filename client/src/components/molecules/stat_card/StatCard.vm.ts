export interface StatCardProps {
  label: string;
  value: string | number;
  unit?: string;
  className?: string;
}

export const useStatCard = (props: StatCardProps) => {
  return props;
};
