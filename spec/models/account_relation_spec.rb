require 'rails_helper'

RSpec.describe AccountRelation, type: :model do
  describe 'validations' do
    it 'has a valid fabricator' do
      relation = Fabricate.build(:account_relation)
      expect(relation).to be_valid
    end

    it 'is invalid without an account' do
      relation = Fabricate.build(:account_relation, account: nil)
      relation.valid?
      expect(relation).to model_have_error_on_field(:account)
    end

    it 'is invalid without a target_account' do
      relation = Fabricate.build(:account_relation, target_account: nil)
      relation.valid?
      expect(relation).to model_have_error_on_field(:target_account)
    end
  end
end
