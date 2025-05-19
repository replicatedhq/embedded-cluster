export interface ImagePushStatus {
  image: string;
  status: 'pending' | 'pushing' | 'complete' | 'failed';
  progress: number;
}