# frozen_string_literal: true

class Api::ActivityIntentsController < Api::BaseController
  def follow
    render json: Account.find_local!(params[:id]), serializer: ActivityPub::FollowIntentSerializer
  end
end
