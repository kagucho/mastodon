import { connect } from 'react-redux';
import Card from '../components/card';

const mapStateToProps = (state, { statusId }) => ({
  card: state.getIn(['cards', statusId], null),
  statusId,
});

export default connect(mapStateToProps)(Card);
