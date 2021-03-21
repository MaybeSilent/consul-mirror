import Route from 'consul-ui/routing/route';

export default class HealthchecksRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    check: 'check',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Node', 'CheckID', 'Notes', 'Output', 'ServiceTags']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return {
      ...this.modelFor(parent),
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
